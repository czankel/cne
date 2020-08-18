// Package containerd implements the runtime interface for the ContainerD Dameon containerd.io
package containerd

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/images"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/containerd/reference"
	"github.com/containerd/containerd/snapshots"

	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
	runspecs "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/czankel/cne/config"
	"github.com/czankel/cne/runtime"
)

// containerdRuntime provides the runtime implementation for the containerd daemon
// For more information about containerd, see: https://github.com/containerd/containerd
type containerdRuntime struct {
	client    *containerd.Client
	context   context.Context
	namespace string
}

type containerdRuntimeType struct {
}

const contextName = "cne"

func init() {
	runtime.Register("containerd", &containerdRuntimeType{})
}

// Runtime Interface

func (r *containerdRuntimeType) Open(confRun config.Runtime) (runtime.Runtime, error) {

	// Validate the provided port
	_, err := os.Stat(confRun.SocketName)
	if err != nil {
		return nil, runtime.Errorf("failed to open runtime socket '%s': %v",
			confRun.SocketName, err)
	}

	client, err := containerd.New(confRun.SocketName)
	if err != nil {
		return nil, runtime.Errorf("failed to open runtime socket '%s': %v",
			confRun.SocketName, err)
	}

	ctrdCtx := namespaces.WithNamespace(context.Background(), confRun.Namespace)

	return &containerdRuntime{
		client:    client,
		context:   ctrdCtx,
		namespace: confRun.Namespace,
	}, nil
}

func (ctrdRun *containerdRuntime) Namespace() string {
	return ctrdRun.namespace
}

func (ctrdRun *containerdRuntime) Close() {
	ctrdRun.client.Close()
}

func (ctrdRun *containerdRuntime) Images() ([]runtime.Image, error) {

	ctrdImgs, err := ctrdRun.client.ListImages(ctrdRun.context)
	if err != nil {
		return nil, runtime.Errorf("ListImages failed: %v", err)
	}

	runImgs := make([]runtime.Image, len(ctrdImgs))
	for i, ctrdImg := range ctrdImgs {
		runImgs[i] = &image{
			ctrdRuntime: ctrdRun,
			ctrdImage:   ctrdImg,
		}
	}

	return runImgs, nil
}

// TODO: ContainerD is not really stable when interrupting an image pull (e.g. using CTRL-C)
// TODO: Snapshots can stay in extracting stage and never complete.

func (ctrdRun *containerdRuntime) PullImage(name string,
	progress chan<- []runtime.ProgressStatus) (runtime.Image, error) {

	var mutex sync.Mutex
	descs := []ocispec.Descriptor{}

	var wg sync.WaitGroup
	wg.Add(1)

	h := images.HandlerFunc(func(ctrdCtx context.Context,
		desc ocispec.Descriptor) ([]ocispec.Descriptor, error) {

		if desc.MediaType != images.MediaTypeDockerSchema1Manifest {
			mutex.Lock()
			found := false
			for _, d := range descs {
				if desc.Digest == d.Digest {
					found = true
					break
				}
			}
			if !found {
				descs = append(descs, desc)
			}
			mutex.Unlock()
		}
		return nil, nil
	})

	pctx, stopProgress := context.WithCancel(ctrdRun.context)
	if progress != nil {
		go func() {
			defer wg.Done()
			defer close(progress)
			updateImageProgress(ctrdRun, pctx, &mutex, &descs, progress)
		}()
	}

	// ignore signals while pulling - see comment above
	signal.Ignore()

	ctrdImg, err := ctrdRun.client.Pull(ctrdRun.context, name,
		containerd.WithPullUnpack, containerd.WithImageHandler(h))

	signal.Reset()

	if progress != nil {
		stopProgress()
		wg.Wait()
	}

	if err == reference.ErrObjectRequired {
		return nil, runtime.Errorf("invalid image name '%s': %v", name, err)
	} else if err != nil {
		return nil, runtime.Errorf("pull image '%s' failed: %v", name, err)
	}

	return &image{
		ctrdRuntime: ctrdRun,
		ctrdImage:   ctrdImg,
	}, nil
}

func (ctrdRun *containerdRuntime) DeleteImage(name string) error {
	imgSvc := ctrdRun.client.ImageService()

	err := imgSvc.Delete(ctrdRun.context, name, images.SynchronousDelete())
	if err != nil {
		return runtime.Errorf("delete image '%s' failed: %v", name, err)
	}

	return nil

}

func (ctrdRun *containerdRuntime) Snapshots(domain [16]byte) ([]runtime.Snapshot, error) {

	snapMap := make(map[string]runtime.Snapshot)
	isParent := make(map[string]bool)
	snapSVC := ctrdRun.client.SnapshotService(containerd.DefaultSnapshotter)
	err := snapSVC.Walk(ctrdRun.context, func(ctx context.Context, info snapshots.Info) error {
		if !isParent[info.Name] {
			snapMap[info.Name] = &snapshot{info: info}
		}
		if info.Parent != "" {
			isParent[info.Parent] = true
		}
		return nil
	})
	if err != nil {
		return nil, runtime.Errorf("failed to get snapshots: %v", err)
	}

	for p := range isParent {
		if _, ok := snapMap[p]; ok {
			delete(snapMap, p)
		}
	}

	var snaps []runtime.Snapshot
	for _, s := range snapMap {
		snaps = append(snaps, s)
	}

	return snaps, nil
}

func (ctrdRun *containerdRuntime) Containers(domain [16]byte) ([]runtime.Container, error) {

	var runCtrs []runtime.Container

	ctrdCtrs, err := ctrdRun.client.Containers(ctrdRun.context)
	if err != nil {
		return nil, runtime.Errorf("failed to get containers: %v", err)
	}

	for _, c := range ctrdCtrs {

		dom, id, err := splitCtrdID(c.ID())
		if err != nil {
			return nil, err
		}
		if dom != domain {
			continue
		}

		img, err := c.Image(ctrdRun.context)
		if err != nil {
			return nil, runtime.Errorf("failed to get image: %v", err)
		}
		spec, err := c.Spec(ctrdRun.context)
		if err != nil {
			return nil, runtime.Errorf("failed to get image spec: %v", err)
		}

		runCtrs = append(runCtrs, &container{
			domain:        dom,
			id:            id,
			image:         &image{ctrdRun, img},
			spec:          spec,
			ctrdRuntime:   ctrdRun,
			ctrdContainer: c,
		})
	}
	return runCtrs, nil
}

func (ctrdRun *containerdRuntime) NewContainer(domain [16]byte, id [16]byte, generation [16]byte,
	img runtime.Image, spec *runspecs.Spec) (runtime.Container, error) {

	return &container{
		domain:        domain,
		id:            id,
		generation:    generation,
		image:         img.(*image),
		spec:          spec,
		ctrdRuntime:   ctrdRun,
		ctrdContainer: nil,
	}, nil
}
