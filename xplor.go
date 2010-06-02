package main

import (
	"os"
	"path"
	"fmt"
//	"strings"
	"sort"

	"goplan9.googlecode.com/hg/plan9/acme"
)

	var root string
	var w *acme.Win
	 
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

	tabs := ""
	for i := 0; i < depth; i++ {
		tabs = tabs + "	"
	}
	for _, v := range names {	
		w.Write("data", []byte(tabs + v + "\n"))
	}
}

//TODO: func getDepth
//TODO: func isExpanded

func readLine(charaddr string) ([]byte, os.Error) {
//TODO: do better about the buf of 512?
	const NBUF = 512	
	var b []byte = make([]byte, NBUF)
	var err os.Error = nil
	addr := "#" + charaddr + "+-"
	err = w.Addr("%s", addr)
	if err != nil {
		w.Write("body", []byte(err.String() + ": " + addr))
		return b, err
	}
	n, err := w.Read("xdata", b)
	if err != nil {
		w.Write("body", []byte(err.String()))
	}	
	
	return b[0:n-1], err
}

func onLook(word string) {
	b, err := readLine(word)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return
	}
	fpath := path.Join(root, string(b))
	fi, err := os.Lstat(fpath)
	if err != nil {
		w.Write("body", []byte(err.String()))
		return
	}
	if !fi.IsDirectory() {
//TODO: send back a Look event?
		return
	}
	addr := "#" + word + "+-#0"
	err = w.Addr("%s", addr)
	if err != nil {
		w.Write("body", []byte(err.String() + addr))
		return
	}	
	printDirContents(fpath, 1)
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
