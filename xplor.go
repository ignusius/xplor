// 2010 - Mathieu Lonjaret

package main

import (
	"os"
//	"bytes"
	"path"
	"fmt"
	"strings"
	"sort"
	"flag"
	"exec"
	//	"bytes"

	"goplan9.googlecode.com/hg/plan9"
	"goplan9.googlecode.com/hg/plan9/acme"
	"bitbucket.org/fhs/goplumb/plumb"
)

var (
	root string
	w *acme.Win
	PLAN9 = os.Getenv("PLAN9")
	unfolded map[int] string
//	chartoline *List 
)

const (
	NBUF = 512
	INDENT = "	"
	dirflag = "+ "
)

type dir struct  {
	charaddr string
	depth int
}
	
func usage() {
	fmt.Fprintf(os.Stderr, "usage: xplor [path] \n")
	flag.PrintDefaults()
	os.Exit(2)
}

//TODO: send error messages to +Errors instead of Stderr? easy sol: start xplor from the acme tag
//TODO: make a closeAll command ?
//TODO: keep a list of unfolded  dirs because it will be faster to go through that list and find the right parent for a file when constructing the fullpath, than going recursively up. After some experimenting, it's not that easy to do, so I may as well make a full rework that uses a tree in memory to map the filesystem tree, rather than go all textual.

func main() {
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()

	switch len(args) {
	case 0:
		root, _ = os.Getwd()
	case 1:
		temp := path.Clean(args[0])
		if temp[0] != '/' {
			cwd, _ := os.Getwd()
			root = path.Join(cwd, temp)
		} else {
			root = temp
		}
	default:
		usage()
	}

	err := initWindow()
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
//	unfolded = make(map[string] int, 1)

	for word := range events() {
		if word == "DotDot" {
			doDotDot()
			continue
		}
		if len(word) >= 3 && word[0:3] == "Win" {
			if PLAN9 != "" {
				cmd := path.Join(PLAN9, "bin/win")
				doExec(word[3:len(word)], cmd)
			}
			continue
		}
// yes, this does not cover all possible cases. I'll do better if anyone needs it.
		if len(word) >= 5 && word[0:5] == "Xplor" {
			cmd, err := exec.LookPath("xplor")
			if err != nil {
				fmt.Fprintf(os.Stderr, err.String())
				continue
			}
			doExec(word[5:len(word)], cmd)
			continue
		}
		if word[0] == 'X' {
			onExec(word[1:len(word)])
			continue
		}
		onLook(word)
	}
}

/*
func initCharToLine(lines int) (err os.Error) {
	b := make([]byte, 64)
	var (
		m, n int
		addr string = "#1+1-"
	)
	chartoline.Init() 

	for i:= 0; i < lines; i++ {
		err = w.Addr("%s", addr)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			os.Exit(1)
		}
		n, err = w.Read("xdata", b)
		if err != nil && err != os.EOF{
			fmt.Fprintf(os.Stderr, err.String())
			os.Exit(1)
		}
		m += n
		print(m, "\n")
		chartoline[i] = m
		addr = "#" + fmt.Sprint(m+1) + "+1-"
	}
	for i:= 0; i < lines; i++ {
		print(i, ": ", chartoline[i], "\n")
	}
	return err
}
*/

func initWindow() os.Error {
	var err os.Error = nil
	w, err = acme.New()
	if err != nil {
		return err
	}

	title := "xplor-" + root
	w.Name(title)
	tag := "DotDot Win Xplor"
	w.Write("tag", []byte(tag))
	_, err = printDirContents(root, 0)
//	initCharToLine(len(newlines))
	return err
}

func printDirContents(dirpath string, depth int) (newlines []int, err os.Error) {
	var m int
	currentDir, err := os.Open(dirpath, os.O_RDONLY, 0644)
	if err != nil {
		return newlines, err
	}
	names, err := currentDir.Readdirnames(-1)
	if err != nil {
		return newlines, err
	}
	currentDir.Close()

	sort.SortStrings(names)
	indents := ""
	for i := 0; i < depth; i++ {
		indents = indents + INDENT
	}
	for _, v := range names {
		// we want to be fast so we assume (for the printing) that any name containing a dot is not a dir
		if !strings.Contains(v, ".") {
			fullpath := path.Join(dirpath, v)
			fi, err := os.Lstat(fullpath)
			if err != nil {
				return newlines, err
			}
			if fi.IsDirectory() {
				m, _ = w.Write("data", []byte(dirflag+indents+v+"\n"))
				newlines = append(newlines, m)
			}
		} else {
			m, _ = w.Write("data", []byte("  "+indents+v+"\n"))
			newlines = append(newlines, m)
		}
	}

	if depth == 0 {
	//lame trick for now to dodge the out of range issue, until my address-foo gets better
		w.Write("body", []byte("\n"))
		w.Write("body", []byte("\n"))
		w.Write("body", []byte("\n"))
	} 

	return newlines, err
}

func readLine(addr string) ([]byte, os.Error) {
	var b []byte = make([]byte, NBUF)
	var err os.Error = nil
	err = w.Addr("%s", addr)
	if err != nil {
		return b, err
	}
	n, err := w.Read("xdata", b)

	// remove dirflag, if any
	if n < 2 {
		return b[0 : n-1], err
	}
	return b[2 : n-1], err
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

/*
func getParent(line string) string {
	d, leaf := getDepth(line)
	var parent string 
	for k, v := range unfolded {
			fullpath := path.Join(k, leaf)
			fi, err := os.Lstat(fullpath)
			if err == nil {
			fmt.Fprintf(os.Stderr, err.String())
		return
	}		
		dist, _ := strconv.Atoi(v.charaddr)
		dist-= pos
		if dist > 0 && dist < min {
			depth, _ := getDepth(b)
			if v.depth == depth - 1 {
				min = dist
				parent = k
			}
		}
	}
	print(parent, "\n")
	return parent
}
*/

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

/*
func addToCharToLine(newlines []int, where int) {
	for i:=0; i<len(newlines); i++ {
		print(newlines[i], " ")
	}
}

//IDEA: get the offset from the total size before and after the folding
func removeFromCharToLine([]newLines, offset) {

}

func addToUnfolded(fullpath string, newdir dir) os.Error {
	_, ok := unfolded[fullpath]
	if ok {
		errstring := fullpath + " already in list"
		return os.NewError(errstring)
	}
	unfolded[fullpath] = newdir
	for k,v := range unfolded {
		print(k, " ", v.charaddr, " ", v.depth, "\n")
	}
	print("\n")
	return nil
}

//TODO: remove all the children as well
func removeFromUnfolded(fullpath string) os.Error {
	for k, _ := range unfolded {
		if strings.Contains(k, fullpath) {
			unfolded[k] = dir{}, false
		}
	}
	for k,v := range unfolded {
		print(k, " ", v.charaddr, " ", v.depth, "\n")
	}
	print("\n")
	return nil
}
*/

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
//	fullpath := path.Join(root, getParent(charaddr), line)
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
//		newlines, _ := printDirContents(fullpath, depth+1)
//		addToCharToLine(newlines, 1)
//		addToUnfolded(fullpath, dir{charaddr, depth})
	} else {
		// fold, ie delete lines below dir until we hit a dir of the same depth
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
//		removeFromUnfolded(charaddr)
	}
}

//TODO: deal with errors
func getFullPath(charaddr string) (fullpath string, err os.Error) {
	// reconstruct full path and print it to Stdout
	addr := "#" + charaddr + "+1-"
	b, err := readLine(addr)
	if err != nil {
		return fullpath, err
	}
	depth, line := getDepth(b)
	fullpath = path.Join(root, getParents(charaddr, depth, 1), line)
//	fullpath = path.Join(root, getParent(charaddr), line)
	return fullpath, err
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
	_, err = printDirContents(root, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		os.Exit(1)
	}
}

/*
//TODO: read about defer
// For this function to work as intended, it needs to be coupled with a
// plumbing rule, such as:
// # start rc from xplor in an acme win at the specified path 
// type is text
// src is xplor
// dst is win
// plumb start win rc -c '@{cd '$data'; exec rc -l}'
func doPlumb(loc string, dst string) {
	var fullpath string
	if loc == "" {
		fullpath = root
	} else {
		var err os.Error
		charaddr := strings.SplitAfter(loc, ",#", 2) 
		fullpath, err = getFullPath(charaddr[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			return
		}
		fi, err := os.Lstat(fullpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			return
		}
		if !fi.IsDirectory() {
			fullpath, _ = path.Split(fullpath)
		}
	}
	// send the fullpath as a win command to the plumber
	port, err := plumb.Open("send", plan9.OWRITE)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
//	defer port.Close()
	err = port.Send(&plumb.Msg{
		Src:  "xplor",
		Dst:  dst,
		WDir: "/",
		Kind: "text",
		Attr: map[string]string{},
		Data: []byte(fullpath),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
	}
	port.Close()
	return
}
*/

func doExec(loc string, cmd string) {
	var fullpath string
	if loc == "" {
		fullpath = root
	} else {
		var err os.Error
		charaddr := strings.SplitAfter(loc, ",#", 2) 
		fullpath, err = getFullPath(charaddr[1])
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			return
		}
		fi, err := os.Lstat(fullpath)
		if err != nil {
			fmt.Fprintf(os.Stderr, err.String())
			return
		}
		if !fi.IsDirectory() {
			fullpath, _ = path.Split(fullpath)
		}
	}
	var args []string = make([]string, 1)
	args[0] = cmd
	fds := []*os.File{os.Stdin, os.Stdout, os.Stderr}
	_, err := os.ForkExec(args[0], args, os.Environ(), fullpath, fds)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return 
	}
	return
}

// on a B2 click event we print the fullpath of the file to Stdout.
// This can come in handy for paths with spaces in it, because the
// plumber will fail to open them.  Printing it to Stdout allows us to do
// whatever we want with it when that happens.
// Also usefull with a dir path: once printed to stdout, a B3 click on
// the path to open it the "classic" acme way.
func onExec(charaddr string) {
	fullpath, err := getFullPath(charaddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.String())
		return
	}
	fmt.Fprintf(os.Stderr, fullpath+"\n")
}

func events() <-chan string {
	c := make(chan string, 10)
	go func() {
		for e := range w.EventChan() {
			switch e.C2 {
			case 'x': // execute in tag
				switch string(e.Text) {
				case "Del":
					w.Ctl("delete")
				case "DotDot":
					c <- "DotDot"
				case "Win":
					tmp := ""
					if e.Flag != 0 {
						tmp = string(e.Loc)
					}
					c <- ("Win" + tmp)
				case "Xplor":
					tmp := ""
					if e.Flag != 0 {
						tmp = string(e.Loc)
					}
					c <- ("Xplor" + tmp)			
				default:
					w.WriteEvent(e)
				}
			case 'X': // execute in body
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
