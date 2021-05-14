package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/czankel/cne/errdefs"
	"github.com/czankel/cne/runtime"
)

// scanLine splits up commands separated by a ',' into multiple command lines
func scanLine(line string) []string {

	line = strings.TrimSpace(line)
	if len(line) == 0 {
		return []string{}
	}

	var cmdLines []string
	for {
		pos := strings.IndexAny(line, ",")
		if pos != -1 {
			if pos > 0 {
				cmdLines = append(cmdLines, strings.TrimSpace(line[:pos]))
			}
			line = strings.TrimSpace(line[pos+1:])
		} else {
			cmdLines = append(cmdLines, line)
			break
		}
	}

	return cmdLines
}

// readCommands reads commands from the io.Reader into a slice of strings
func readCommands(reader io.Reader) ([]string, error) {

	var cmdLines []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		cmdLines = append(cmdLines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, errdefs.InvalidArgument("unable to read line: %v", err)
	}
	return cmdLines, nil
}

// sizeToSIString converts the provide integer value to a SI size string from the 10^3x exponent
func sizeToSIString(sz int64) string {
	const unit = 1000
	b := sz
	if b < 0 {
		b = -b
	}
	if b < unit {
		return fmt.Sprintf("%dB", b)
	}

	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f%cB", float64(sz)/float64(div), "kMGTPE"[exp])
}

// timeToAgoString converts the timespan from the provided time to the current time to a string
// in the formwat "T {year|month|hour}[s] ago". Future dates will return 'future'
func timeToAgoString(t time.Time) string {

	now := time.Now()
	if now.Before(t) {
		return "future"
	}

	diff := now.Sub(t)
	hours := diff.Hours()
	years := int(hours / 365 / 24)

	if years == 1 {
		return "one year ago"
	} else if years > 1 {
		return strconv.Itoa(years) + " years ago"
	}

	months := int(hours / 30.5)
	if months == 1 {
		return "one month ago"
	} else if months >= 1 {
		return strconv.Itoa(months) + " months ago"
	}

	if int(hours) == 1 {
		return "one hour ago"
	} else if hours > 1 {
		return strconv.Itoa(int(hours)) + " hours ago"
	}

	mins := diff.Minutes()
	if int(mins) == 1 {
		return "one minute ago"
	} else if mins > 1 {
		return strconv.Itoa(int(mins)) + " minutes ago"
	}

	return "seconds ago"
}

// printValueElem prints the provided value as two columns for name and value content.
// Struct  Each element is printed as a single row with the provided prefix for the name field
//         For nested structures, the field names of each substructure are concatenated by '.'
//         Use the output:"-" tag to omit a field.
// Map     Similar to Struct, but using the keys as the prefix instead of the structure elements.
// <other> Printed as two columns using the prefix as the name for the value content.
// flat outputs a slice in the [ ... ] format. It will only flatten the final slice ([][])
func printValueElem(w *tabwriter.Writer, prefix string, elem reflect.Value, flat bool) {

	kind := elem.Kind()

	cPrefix := prefix
	if prefix != "" && (kind == reflect.Struct || kind == reflect.Map || kind == reflect.Slice) {
		prefix = prefix + "/"
	}

	if kind == reflect.Struct {
		elemType := elem.Type()

		if elemType == reflect.TypeOf(time.Time{}) {
			t := elem.Interface().(time.Time).Format(time.RFC3339Nano)
			fmt.Fprintf(w, "%s\t%v\n", prefix, t)
			return
		}

		for i := 0; i < elem.NumField(); i++ {
			elemField := elem.Field(i)
			if !elemField.CanInterface() {
				break
			}
			if elemType.Field(i).Tag.Get("output") != "-" {
				flat := elemType.Field(i).Tag.Get("output") == "flat"
				printValueElem(w, prefix+elemType.Field(i).Name, elemField, flat)
			}
		}
	} else if kind == reflect.Map {
		m := elem.MapKeys()
		keys := make([]string, len(m))
		for i := 0; i < len(m); i++ {
			keys[i] = m[i].String()
		}
		sort.Strings(keys)
		for _, k := range keys {
			printValueElem(w, prefix+k, elem.MapIndex(reflect.ValueOf(k)), false)
		}
	} else if kind == reflect.Slice {
		if elem.Len() == 0 {
			return
		}
		if flat && elem.Index(0).Kind() != reflect.Slice && elem.Index(0).CanInterface() {
			fmt.Fprintf(w, "%s\t%v\n", cPrefix, elem.Interface())
		} else {
			for i := 0; i < elem.Len(); i++ {
				printValueElem(w, prefix+strconv.Itoa(i), elem.Index(i), flat)
			}
		}
	} else if kind == reflect.Ptr {
		printValueElem(w, prefix, elem.Elem(), false)
	} else if elem.CanInterface() {
		fmt.Fprintf(w, "%s\t%v\n", prefix, elem.Interface())
	}
}

// printValue prints the content of the provided value in two columns.
//  struct: field name, value
//  map:    key, value
//  slice:  index, value
//  <type>: prefix, value
func printValue(fieldHdr string, valueHdr string, prefix string, value interface{}) {

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 0, 1, ' ', 0)
	defer w.Flush()

	fmt.Fprintf(w, "%s\t%s\n", strings.ToUpper(fieldHdr), strings.ToUpper(valueHdr))
	printValueElem(w, prefix, reflect.ValueOf(value), false)
}

// printList prints a slice of structures using the field names as the header
func printList(list interface{}) {

	if reflect.TypeOf(list).Kind() != reflect.Slice {
		panic("provided argument must be of the type: slice")
	}
	if reflect.TypeOf(list).Elem().Kind() != reflect.Struct {
		panic("provided argument must be of the type: slice of structures")
	}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 0, 1, ' ', 0)
	defer w.Flush()

	format := "%s"
	hdr := reflect.TypeOf(list).Elem()
	for i := 0; i < hdr.NumField(); i++ {
		if hdr.Field(i).Tag.Get("output") != "-" {
			fmt.Fprintf(w, format, strings.ToUpper(hdr.Field(i).Name))
			format = "\t%s"
		}
	}
	fmt.Fprintf(w, "\n")

	items := reflect.ValueOf(list)
	for i := 0; i < items.Len(); i++ {
		format = "%s"
		item := items.Index(i)
		for j := 0; j < item.NumField(); j++ {
			if hdr.Field(j).Tag.Get("output") == "-" {
				continue
			}
			fmt.Fprintf(w, format, item.Field(j).Interface())
			format = "\t%s"
		}
		fmt.Fprintf(w, "\n")
	}
}

// showImageProgress displays the progress of sequential or parallel jobs
// Use this as a callback function in calls that provide a progress feedback
func showImageProgress(progress <-chan []runtime.ProgressStatus) {

	lines := 0
	ticks := 0

	statCached := make(map[string]runtime.ProgressStatus)
	statRefs := []string{}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 0, 1, ' ', 0)
	defer w.Flush()

	for statUpdate := range progress {
		for _, status := range statUpdate {
			if _, ok := statCached[status.Reference]; !ok {
				statRefs = append(statRefs, status.Reference)
			}
			statCached[status.Reference] = status
		}
		for ; lines > 0; lines = lines - 1 {
			fmt.Fprintf(w, "\033[1A\033[2K")
		}
		lines = len(statRefs)
		for _, ref := range statRefs {

			status := statCached[ref]

			decoded := strings.Index(ref, ":")
			if decoded > 0 {
				ref = ref[decoded+1:]
			}
			if status.Status == runtime.StatusRunning {
				if status.Offset == status.Total {
					fmt.Fprintf(w, "%s: Extracting %c\n",
						ref[:12],
						"-\\|/"[ticks&3])
				} else {
					fmt.Fprintf(w, "%s: Downloading (%s / %s)\n",
						ref[:12],
						sizeToSIString(status.Offset),
						sizeToSIString(status.Total))
				}
			} else {
				fmt.Fprintf(w, "%s: %s\n", ref[:12], strings.Title(status.Status))
			}
		}
		w.Flush()
		ticks = ticks + 1
	}
}

// showBuildProgress displays the progress of sequential or parallel jobs
// Use this as a callback function in calls that provide a progress feedback
func showBuildProgress(progress <-chan []runtime.ProgressStatus) {

	lines := 0
	ticks := 0

	statCached := make(map[string]runtime.ProgressStatus)
	statRefs := []string{}

	w := new(tabwriter.Writer)
	w.Init(os.Stdout, 8, 0, 1, ' ', 0)
	defer w.Flush()

	for statUpdate := range progress {
		for _, status := range statUpdate {
			if _, ok := statCached[status.Reference]; !ok {
				statRefs = append(statRefs, status.Reference)
			}
			statCached[status.Reference] = status
		}
		for ; lines > 0; lines = lines - 1 {
			fmt.Fprintf(w, "\033[1A\033[2K")
		}
		lines = len(statRefs)
		for _, ref := range statRefs {

			status := statCached[ref]

			decoded := strings.Index(ref, ":")
			if decoded > 0 {
				ref = ref[decoded+1:]
			}
			reflen := len(ref)
			if reflen > 12 {
				reflen = 12
			}
			if status.Status == runtime.StatusRunning {
				fmt.Fprintf(w, "[%s] %s\n",
					ref[:reflen],
					status.Details)
			} else {
				fmt.Fprintf(w, "[%s] %s\n", ref[:reflen], strings.Title(status.Status))
			}
		}
		w.Flush()
		ticks = ticks + 1
	}
}
