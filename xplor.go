// 2010 - Mathieu Lonjaret

package main

import (
	"os"
	"path"
	"fmt"
	"strings"
	"sort"
	"flag"

	"goplan9.googlecode.com/hg/plan9"
	"goplan9.googlecode.com/hg/plan9/acme"
	"bitbucket.org/fhs/goplumb/plumb"
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

	err := initWindow()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}

	for word := range events() {
		if word == "DotDot" {
			doDotDot()
			continue
		}
		if word[0] == 'X' {
			onExec(word[1:len(word)])
			continue
		}
		onLook(word)
	}
}

func initWindow() os.Error {
	var err os.Error = nil
	w, err = acme.New()
	if err != nil {
		return err
	}

	title := "xplor-" + root
	w.Name(title)
	tag := "DotDot"
	w.Write("tag", []byte(tag))
	return printDirContents(root, 0)
}

func printDirContents(path string, depth int) os.Error {
	currentDir, err := os.Open(path, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		return err
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
	return err
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
			fmt.Fprintf(os.Stderr, err.String())
			os.Exit(1)
		}
		newdepth, line := getDepth(b)
		if newdepth < depth {
			fullpath := path.Join(getParents(charaddr, newdepth, prevline), line)
			return fullpath
		}
		prevline++
		addr = "#" + charaddr + "-" + fmt.Sprint(prevline-1)
	}
	return ""
}

//TODO: maybe break this one in a fold and unfold functions
func onLook(charaddr string) {
	// reconstruct full path and check if file or dir
	addr := "#" + charaddr + "+1-"
	b, err := readLine(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	depth, line := getDepth(b)
	fullpath := path.Join(root, getParents(charaddr, depth, 1), line)
	fi, err := os.Lstat(fullpath)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}

	if !fi.IsDirectory() {
		// not a dir -> send that file to the plumber
		port, err := plumb.Open("send", plan9.OWRITE)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			return
		}
		defer port.Close()
		port.Send(&plumb.Msg{
			Src:  "xplor",
			Dst:  "",
			WDir: "/",
			Kind: "text",
			Attr: map[string]string{},
			Data: []byte(fullpath),
		})
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

func doDotDot() {
	// blank the window
	err := w.Addr("0,$")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		os.Exit(1)
	}
	w.Write("data", []byte(""))

	// restart from ..
	root = path.Clean(root + "/../")
	title := "xplor-" + root
	w.Name(title)
	err = printDirContents(root, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		os.Exit(1)
	}
}

// on a B2 click event we print the fullpath of the file to Stdout.
// This can come in handy for paths with spaces in it, because the
// plumber will fail to open them.  Printing it to Stdout allows us to do
// whatever we want with it when that happens.
// Also usefull with a dir path: once printed to stdout, a B3 click on
// the path to open it the "classic" acme way.
func onExec(charaddr string) {
	// reconstruct full path and print it to Stdout
	addr := "#" + charaddr + "+1-"
	b, err := readLine(addr)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	depth, line := getDepth(b)
	fullpath := path.Join(root, getParents(charaddr, depth, 1), line)
	fmt.Fprintf(os.Stdout, fullpath+"\n")
}

func events() <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			switch e.C2 {
			case 'x': // execute in body
				switch string(e.Text) {
				case "Del":
					w.Ctl("delete")
				case "DotDot":
					c <- "DotDot"
				default:
					w.WriteEvent(e)
				}
			case 'X': // execute in tag
				c <- ("X" + fmt.Sprint(e.OrigQ0))
			case 'l': // button 3 in tag
				// let the plumber deal with it
				w.WriteEvent(e)
			case 'L': // button 3 in body
				w.Ctl("clean")
				//ignore expansions
				if e.OrigQ0 != e.OrigQ1 {
					continue
				}
				c <- fmt.Sprint(e.OrigQ0)
			}
		}
		w.CloseFiles()
		close(c)
	}()
	return c
}
