// 2010 - Mathieu Lonjaret

package main

import (
	"os"
	"path"
	"fmt"
	"strings"
	"sort"
	"flag"

	"goplan9.googlecode.com/hg/plan9/acme"
)

var root string
var w *acme.Win
var INDENT string = "	"
var PLAN9 = os.Getenv("PLAN9")

const NBUF = 512

func usage() {
	fmt.Fprintf(os.Stderr, "usage: xplor [path] \n")
	flag.PrintDefaults()
	os.Exit(2)
}

//TODO: send error messages to +Errors instead of Stderr?

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	switch len(args) {
	case 0:
		root, _ = os.Getwd()
	case 1:
		root = args[0]
	default:
		usage()
	}

	initWindow()

	for word := range events() {
		onLook(word)
	}
}

func initWindow() {
	var err os.Error
	w, err = acme.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}

	title := "xplor-" + root
	w.Name(title)

	printDirContents(root, 0)
}

func printDirContents(path string, depth int) {
	currentDir, err := os.Open(path, os.O_RDONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	currentDir.Close()

	sort.SortStrings(names)
	indents := ""
	for i := 0; i < depth; i++ {
		indents = indents + INDENT
	}
	for _, v := range names {
		w.Write("data", []byte(indents+v+"\n"))
	}

	//lame trick for now to dodge the out of range issue, until my address-foo gets better
	if depth == 0 {
		w.Write("body", []byte("\n"))
		w.Write("body", []byte("\n"))
		w.Write("body", []byte("\n"))
	}
}

func readLine(addr string) ([]byte, os.Error) {
	var b []byte = make([]byte, NBUF)
	var err os.Error = nil
	err = w.Addr("%s", addr)
	if err != nil {
		return b, err
	}
	n, err := w.Read("xdata", b)

	return b[0 : n-1], err
}

func getDepth(line []byte) (depth int, trimedline string) {
	trimedline = strings.TrimLeft(string(line), INDENT)
	depth = (len(line) - len(trimedline)) / len(INDENT)
	return depth, trimedline
}

func isFolded(charaddr string) (bool, os.Error) {
	var err os.Error = nil
	var b []byte
	addr := "#" + charaddr + "+1-"
	b, err = readLine(addr)
	if err != nil {
		return true, err
	}
	depth, _ := getDepth(b)
	addr = "#" + charaddr + "+-"
	b, err = readLine(addr)
	if err != nil {
		return true, err
	}
	nextdepth, _ := getDepth(b)
	return (nextdepth <= depth), err
}

func getParents(charaddr string, depth int, prevline int) string {
	var addr string
	if depth == 0 {
		return ""
	}
	if prevline == 1 {
		addr = "#" + charaddr + "-+"
	} else {
		addr = "#" + charaddr + "-" + fmt.Sprint(prevline-1)
	}
	for {
		b, err := readLine(addr)
		if err != nil {
			w.Write("body", []byte(err.String()))
			return ""
		}
		newdepth, line := getDepth(b)
		if newdepth < depth {
			fullpath := path.Join(getParents(charaddr, newdepth, prevline), "/", line)
			return fullpath
		}
		prevline++
		addr = "#" + charaddr + "-" + fmt.Sprint(prevline-1)
	}
	return ""
}

func onLook(charaddr string) {
	// reconstruct full path and check if file or dir
	addr := "#" + charaddr + "+1-"
	b, err := readLine(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	depth, line := getDepth(b)
	fullpath := path.Join(root, "/", getParents(charaddr, depth, 1), "/", line)
	fi, err := os.Lstat(fullpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}

	if !fi.IsDirectory() {
		// send that file to B (ie open it)
		if len(PLAN9) == 0 {
			fmt.Fprintf(os.Stderr, "$PLAN9 not defined \n")
			return
		}
		var args2 []string = make([]string, 2)
		args2[0] = path.Join(PLAN9 + "/bin/B")
		args2[1] = fullpath
		fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
		os.ForkExec(args2[0], args2, os.Environ(), "", fds)
		return
	}

	folded, err := isFolded(charaddr)
	if err != nil {
		fmt.Fprint(os.Stderr, err.String())
		return
	}
	if folded {
		// print dir contents
		addr = "#" + charaddr + "+2-1-#0"
		err = w.Addr("%s", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String()+addr)
			return
		}
		printDirContents(fullpath, depth+1)
	} else {
		// unfold, ie delete lines below dir until we hit a dir of the same depth
		addr = "#" + charaddr + "+-"
		nextdepth := depth + 1
		nextline := 1
		for nextdepth > depth {
			err = w.Addr("%s", addr)
			if err != nil {
				fmt.Fprint(os.Stderr, err.String())
				return
			}
			b, err = readLine(addr)
			if err != nil {
				fmt.Fprint(os.Stderr, err.String())
				return
			}
			nextdepth, _ = getDepth(b)
			nextline++
			addr = "#" + charaddr + "+" + fmt.Sprint(nextline-1)
		}
		nextline--
		addr = "#" + charaddr + "+-#0,#" + charaddr + "+" + fmt.Sprint(nextline-2)
		err = w.Addr("%s", addr)
		if err != nil {
			fmt.Fprint(os.Stderr, err.String())
			return
		}
		w.Write("data", []byte(""))
	}
}

func events() <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			switch e.C2 {
			case 'x', 'X': // execute
				if string(e.Text) == "Del" {
					w.Ctl("delete")
				}
				w.WriteEvent(e)
			case 'l': // in the tag, let the plumber deal with it
				w.WriteEvent(e)
			case 'L': // look
				w.Ctl("clean")
				//ignore expansions
				if e.OrigQ0 != e.OrigQ1 {
					continue
				}
				msg := fmt.Sprint(e.OrigQ0)
				c <- msg
			}
		}
		w.CloseFiles()
		close(c)
	}()
	return c
}
