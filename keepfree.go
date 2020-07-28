package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

type FileDates map[string]map[uint64]int64

func (fd *FileDates)Print(){
	for w,m := range (*fd){
		fmt.Printf("----\n%s\n",w)
		dates:=make([]uint64,len((*fd)[w]))
		i:=0
		for k,_:=range (*fd)[w] {
			dates[i]=k
			i++
		}
		sort.Slice(dates,func(i, j int) bool { return dates[i] < dates[j] })
		for _,d := range dates {
			s:=m[d]
			fmt.Printf("  %s  %d\n",time.Unix(int64(d),0).Format("2006-01-02 15:04:05"),s)
		}
	}
}

func work(cfg *CFG, fd *FileDates, w string, toFree int64) {
	nfd := make(map[uint64]int64)
	shifting:=0
	l:=len((*fd)[w])
	dates:=make([]uint64,l)
	i:=0
	for k,_:=range (*fd)[w] {
		dates[i]=k
		i++
	}
	sort.Slice(dates,func(i, j int) bool { return dates[i] < dates[j] })
	i=0
	var sum int64=0
	var deleteBelow uint64=0
	if len(dates)>0 {
		for sum < toFree {
			fmt.Printf("sum=%d added=%d from %s\n",sum, (*fd)[w][dates[i]],
				time.Unix(int64(dates[i]),0).Format("2006-01-02 15:04:05"))
			sum+=(*fd)[w][dates[i]]
			i++
			if i>= len(dates){
				i=len(dates)-1
				break
			}
		}
		deleteBelow=dates[i]
	}	
	fmt.Printf("deleting files older than %s\n",
		time.Unix(int64(deleteBelow),0).Format("2006-01-02 15:04:05"))
	var mask uint64=0
	mask=^mask
	reduceNfd:=func(){
		shifting++
		fmt.Printf("mask now shifted %d left\n",shifting)
		mask=0
		mask=(^mask << shifting)
		for k,v:=range nfd {
			k2:=k & mask
			if k2 != k {
				nfd[k2]+= v
				delete(nfd,k)
			}
		}
	}
	fwf := func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			if len(nfd) > cfg.BinCount {
				reduceNfd()
			}
			return nil
		}
		if info.Size() <= 0 {
			return nil
		}
		mt:=info.ModTime()
		mts:=uint64(mt.Unix())
		
		if mts < deleteBelow {
			fmt.Printf("deleting %s\n",path)
			os.Remove(path)
		} else {
			// fmt.Printf("size %d from %s to %x\n", info.Size(),path, mts & mask)
			nfd[mts & mask]+=info.Size()
		}
		return nil
	}
	for _,s:= range cfg.Dirs[w].Scan{
		filepath.Walk(s,fwf)
	}
	// fmt.Println("---- before change")
	// fd.Print()
	(*fd)[w]=nfd
	// fmt.Println("---- changed nfd")
	// fd.Print()
}

func keepfree() {
	fmt.Println("keepfree starting")
	defer fmt.Println("keepfree stopped")
	cfg := GetCfg("keepfree.cfg")
	// fmt.Printf("got %#v\n", cfg)
	fd := make(FileDates)
	RetrieveData(cfg.Datastorage, &fd)
	defer StoreData(cfg.Datastorage, fd)
	allDirs := map[string]int{}
	for w, _ := range cfg.Dirs {
		allDirs[w] = 1
		if fd[w] == nil {
			fd[w] = make(map[uint64]int64)
		} else {
			if len(fd[w]) < 2 {
				work(cfg, &fd, w, 0)
			}
		}
	}
	// fmt.Printf("fd == %#v\n",fd)
	// fd.Print()
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
	for t := range tick {
		fmt.Println(t.Format("15:04:05"))
		for w, _ := range cfg.Dirs {
			var stat syscall.Statfs_t
			err := syscall.Statfs(w, &stat)
			if err != nil {
				log.Fatal(err.Error())
			}
			bAvail := int64(stat.Bavail) * stat.Bsize
			if bAvail < freeSizes[w] {
				fmt.Println("need to delete files")
				work(cfg, &fd, w, freeSizes[w]-bAvail)
				StoreData(cfg.Datastorage, fd)
			}
		}
	}
}

func main() {
	keepfree()
}
