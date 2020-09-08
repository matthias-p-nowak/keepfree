package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"log/syslog"
	"time"
)

// where, which date, how much
type FileDates map[string]map[uint64]int64


var (
	slog syslog.Writer
	lock sync.RWMutex
)

// printing that info as a debug information
func (fd *FileDates) Print() {
	lock.RLock()
	for w, m := range *fd {
		fmt.Printf("----\n%s\n", w)
		// maps are normally unsorted, need to sort stuff
		dates := make([]uint64, len((*fd)[w]))
		i := 0
		for k, _ := range (*fd)[w] {
			dates[i] = k
			i++
		}
		// sort the dates
		sort.Slice(dates, func(i, j int) bool { return dates[i] < dates[j] })
		// finally printing
		for _, d := range dates {
			s := m[d]
			fmt.Printf("  %s  %d\n", time.Unix(int64(d), 0).Format("2006-01-02 15:04:05"), s)
		}
	}
	lock.RUnlock()
}

// starting to remove files to free place
// configuration, dates
// the place to walk free
// the old files to delete
func work(cfg *CFG, fd *FileDates, w string, toFree int64) {
	lock.Lock()
	// place to store the new dates
	nfd := make(map[uint64]int64)
	shifting := 0 // size of the bin
	l := len((*fd)[w])
	dates := make([]uint64, l)
	i := 0
	for k, _ := range (*fd)[w] {
		dates[i] = k
		i++
	}
	sort.Slice(dates, func(i, j int) bool { return dates[i] < dates[j] })
	i = 0
	var sum int64 = 0
	var deleteBelow uint64 = 0
	if len(dates) > 0 {
		for sum < toFree {
			/*
			fmt.Printf("sum=%d added=%d from %s\n", sum, (*fd)[w][dates[i]],
				time.Unix(int64(dates[i]), 0).Format("2006-01-02 15:04:05"))
			*/
			sum += (*fd)[w][dates[i]]
			i++
			if i >= len(dates) {
				i = len(dates) - 1
				break
			}
		}
		deleteBelow = dates[i]
	}
	str:=fmt.Sprintf("deleting files older than %s",
		time.Unix(int64(deleteBelow), 0).Format("2006-01-02 15:04:05"))
	slog.Info(str)
	// using mask to put file sizes into bins
	var mask uint64 = 0
	mask = ^mask // makes all ones
	// closure that uses variable from this function
	// reduceNfd reduces the number of bins by doubling the bin length
	reduceNfd := func() {
		shifting++
		fmt.Printf("mask now shifted %d left\n", shifting)
		mask = 0
		mask = (^mask << shifting)
		for k, v := range nfd {
			k2 := k & mask
			if k2 != k {
				nfd[k2] += v
				delete(nfd, k)
			}
		}
	}
	// walker function
	fwf := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			// when encountering directories, look at the bin size
			if len(nfd) > cfg.BinCount {
				reduceNfd()
			}
		}
		// look at the modification time
		mt := info.ModTime()
		mts := uint64(mt.Unix())
		
		if mts < deleteBelow {
			// fmt.Printf("deleting %s (%s)\n", path,time.Unix(int64(mts), 0).Format("2006-01-02 15:04:05"))
			err:=os.Remove(path)
			if err != nil {
				// fmt.Printf("tried %s (%s),but %s\n", path,time.Unix(int64(mts), 0).Format("2006-01-02 15:04:05"),err)
			}
		} else {
			// fmt.Printf("size %d from %s to %x\n", info.Size(),path, mts & mask)
			nfd[mts&mask] += info.Size()
		}
		return nil
	}
	for _, s := range cfg.Dirs[w].Scan {
		filepath.Walk(s, fwf)
	}
	// fmt.Println("---- before change")
	// fd.Print()
	(*fd)[w] = nfd
	// fmt.Println("---- changed nfd")
	// fd.Print()
	lock.Unlock()
}

func keepfree(cfg *CFG, fd FileDates) {
	defer fmt.Println("keepfree stopped")
	allDirs := map[string]int{}
	for w, _ := range cfg.Dirs {
		allDirs[w] = 1
		if fd[w] == nil {
			fd[w] = make(map[uint64]int64)
		} 
	}
	for w, _ := range fd {
		if allDirs[w] == 0 {
			delete(fd, w)
		}
	}
	freeSizes := map[string]int64{}
	for w, _ := range cfg.Dirs {
		freeSizes[w] = cfg.FreeSize(w)
	}
	tick := time.Tick(time.Duration(cfg.Interval) * time.Second)
	for _ = range tick {
		// fmt.Println(t.Format("15:04:05"))
		for w, _ := range cfg.Dirs {
			var stat syscall.Statfs_t
			err := syscall.Statfs(w, &stat)
			if err != nil {
				log.Fatal(err.Error())
			}
			bAvail := int64(stat.Bavail) * stat.Bsize
			if bAvail < freeSizes[w] {
				// fmt.Println("need to delete files")
				work(cfg, &fd, w, freeSizes[w]-bAvail)
				StoreData(cfg.Datastorage, fd)
			}
		}
	}
}

// prepare and then wait for signals
func main() {
	fmt.Println("https://github.com/matthias-p-nowak/keepfree")
	slog,err:=syslog.New(syslog.LOG_LOCAL0,"keepfree")
	if err != nil { log.Fatal(err)}
	// cmd line prep
	cfgName := flag.String("c", "keepfree.cfg", "the configuration file controlling keep-free")
	pidLoc := flag.String("p", "", "the location for the pid file")
	printCfg := flag.Bool("e", false, "print the configuration and exit")
	dbLoc := flag.String("d", "", "the location of the keep free database")
	flag.Parse()
	// getting config
	cfg := GetCfg(*cfgName)
	// overwrite with command line stuff
	if len(*pidLoc) > 0 {
		cfg.PidFile = *pidLoc
	}
	if len(*dbLoc) > 0 {
		cfg.Datastorage = *dbLoc
	}
	if *printCfg {
		cfg.PrintCfg()
		os.Exit(0)
	}
	pid := os.Getpid()
	if len(cfg.PidFile) > 0 {
		str := fmt.Sprintf("%d\n", pid)
		ioutil.WriteFile(cfg.PidFile, []byte(str), 0644)
	}
	slog.Info(fmt.Sprintf("pid=%d",pid))
	fd := make(FileDates)
	// maps are by reference...
	RetrieveData(cfg.Datastorage, &fd)
	go keepfree(cfg,fd)
	 c := make(chan os.Signal, 1)
  signal.Notify(c,syscall.SIGINT,syscall.SIGHUP,syscall.SIGTERM)
  for s:= range c {
    fmt.Printf("Got signal: %#v\n", s) 
    switch(s){
			case syscall.SIGINT:
				StoreData(cfg.Datastorage,&fd)
				os.Exit(0)
			case syscall.SIGHUP:
				fd.Print()
		} 
  }
	
}
