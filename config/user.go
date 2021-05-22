package config

import (
	"os"
	"os/user"
	"strconv"
)

// The User is technically its own first-class object similar to
// Project and Config. However, since the configuration can define
// the User, it includes a function for returning the User
// credentials rather than being part of it.

const defaultShell = "/bin/bash"

type User struct {
	Username  string
	Groupname string
	Shell     string
	IsSudo    bool
	IsSuid    bool
	UID       uint32
	GID       uint32
	HomeDir   string
	Pwd       string
	BuildUID  uint32
	BuildGID  uint32
}

func getProcessUser() (User, error) {

	usr, err := user.Current()
	if err != nil {
		return User{}, err
	}

	uid := os.Getuid()
	gid := os.Getgid()

	isSuid := uid != os.Geteuid()
	isSudo := false
	username := usr.Username
	sudo_user := os.Getenv("SUDO_USER")
	if sudo_user != "" {
		isSudo = true
		username = sudo_user
		uid, _ = strconv.Atoi(os.Getenv("SUDO_UID"))
		gid, _ = strconv.Atoi(os.Getenv("SUDO_GID"))
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = defaultShell
	}

	groupname := username
	group, err := user.LookupGroupId(strconv.Itoa(gid))
	if err != nil {
		groupname = group.Name
	}

	pwd, err := os.Getwd()
	if err != nil {
		pwd = usr.HomeDir
	}

	user := User{
		Username:  username,
		Groupname: groupname,
		IsSudo:    isSudo,
		IsSuid:    isSuid,
		HomeDir:   usr.HomeDir,
		Pwd:       pwd,
		Shell:     shell,
		UID:       uint32(uid),
		GID:       uint32(gid),
		BuildUID:  uint32(uid),
		BuildGID:  uint32(gid),
	}

	return user, nil
}
