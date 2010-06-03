package main

import (
	"os"
	"path"
	"fmt"
	"strings"
	"sort"

	"goplan9.googlecode.com/hg/plan9/acme"
)

	var root string
	var w *acme.Win
	var INDENT string = "	"

//TODO: send error messages to error win, not to xplor win.	 
func main() {

	initWindow()
	
	for word := range events() {
		go onLook(word) 
	}
}

func initWindow() {
	var err os.Error
	w, err = acme.New()
	if err != nil {
		print(err.String());
		return 
	}

	root, _ = os.Getwd()
	title := path.Join("/Xplor/",root)
	w.Name(title)

	printDirContents(root, 0)
	
	//add an extra line to deal with out of range addresses for now
	w.Write("body", []byte("\n"))
	
}

func printDirContents(path string, depth int) {
	currentDir, err := os.Open(path, os.O_RDONLY, 0644)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return 
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return 
	}
	currentDir.Close();
	
	sort.SortStrings(names)

	indents := ""
	for i := 0; i < depth; i++ {
		indents = indents + INDENT
	}
	for _, v := range names {	
		w.Write("data", []byte(indents + v + "\n"))
	}
}

//TODO: func isUnfolded

func readLine(addr string) ([]byte, os.Error) {
//TODO: do better about the buf of 512?
	const NBUF = 512	
	var b []byte = make([]byte, NBUF)
	var err os.Error = nil
	fmt.Fprint(os.Stdout, "readLine addr: " + addr + "\n")
//	w.Write("body", []byte("readLine addr: " + addr + "\n"))
	err = w.Addr("%s", addr)
	if err != nil {
		return b, err
	}
	n, err := w.Read("xdata", b)
	
	return b[0:n-1], err
}

func getDepth(line []byte) (depth int, trimedline string) {
	trimedline = strings.TrimLeft(string(line), INDENT)
	depth = (len(line) - len(trimedline)) / len(INDENT)
	return depth, trimedline
}

func getParents(charaddr string, depth int, prevline int) string {
	var addr string
	if depth == 0 {
		return ""
	}
	if prevline == 1 {
		addr = "#" + charaddr + "-+"
	} else {
		addr = "#" + charaddr + "-" + fmt.Sprint(prevline - 1)
	}
	for ;; {
		b, err := readLine(addr)
		if err != nil {
			w.Write("body", []byte(err.String()))
			return ""
		}
		newdepth, line := getDepth(b)
		fmt.Fprint(os.Stdout, fmt.Sprint(newdepth) + ", " + line + "\n")
		if newdepth < depth {
			fullpath := path.Join(getParents(charaddr, newdepth, prevline), "/", line)
			fmt.Fprint(os.Stdout, fullpath + "\n")
			return fullpath
		}
		prevline++
		addr = "#" + charaddr + "-" + fmt.Sprint(prevline - 1)
	}
	return ""
}

func onLook(word string) {
	addr := "#" + word + "+1-"
	b, err := readLine(addr)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return
	}
	depth, line := getDepth(b)
	fmt.Fprint(os.Stdout, fmt.Sprint(depth) + ", " + line + "\n")
	fullpath := path.Join(root, "/", getParents(word, depth, 1), "/", line)
	fi, err := os.Lstat(fullpath)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return
	}
	if !fi.IsDirectory() {
//TODO: send back a Look event?
		fmt.Fprint(os.Stdout, "Not a dir \n")
		return
	}
	addr = "#" + word + "+2-1-#0"
	fmt.Fprint(os.Stdout, "onLook addr: " + addr + "\n")
	err = w.Addr("%s", addr)
	if err != nil {
		w.Write("body", []byte(err.String() + addr))
		return
	}	
	printDirContents(fullpath, depth + 1)
}

func events() <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			switch e.C2 {
			case 'x', 'X':	// execute
				if string(e.Text) == "Del" {
					w.Ctl("delete")
				}
				w.WriteEvent(e)
			case 'l': // in the tag, let the plumber deal with it
				w.WriteEvent(e)
			case 'L':	// look
				w.Ctl("clean")
			/*
				msg := fmt.Sprint(e.Q0) + "," + fmt.Sprint(e.Q1) + "," + fmt.Sprint(e.OrigQ0) + "," + fmt.Sprint(e.OrigQ1)
				w.Write("body", []byte(msg))
				continue
			*/
				//disallow expansions to avoid weird addresses for now
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
