package main

/*
 * reading and preparing the configuration structure 
 */
 
import (
  "fmt"
  "gopkg.in/yaml.v2"
  "io/ioutil"
  "log"
)

/*
 * The structure that is read from Yaml configuration file
 */
type CFG struct {
  Datastorage string  `yaml:"datastorage"`
  Interval int     `yaml:"interval"`
  BinCount int `yaml:"bincount"`
  Dirs     map[string]_Dirs
}
/*
 * the different partitions to keep free space
 */
type _Dirs struct {
  Scan      []string `yaml:"scan"`
  Free      string   `yaml:"free"`
}

func (cfg *CFG) FreeSize(watch string) (d int64){
  v:=cfg.Dirs[watch]
  var u string
  fmt.Sscanf(v.Free,"%d%s",&d,&u)
  for _,val := range u {
    switch val {
      case 'k':
        d *= 1024
      case 'M':
        d *= 1024*1024
      case 'G':
        d *= 1024*1024*1024
    }
  }
  return
}

/*
 * reading configuration from a yaml file
 */
func GetCfg(filename string) (cfg *CFG) {
  cfg = new(CFG)
  cfg.BinCount=2048
  cfg.Interval=10
  data, err := ioutil.ReadFile(filename)
  if err != nil {
    log.Fatal(err)
  }
  err = yaml.Unmarshal(data, cfg)
  if err != nil {
    log.Fatal(err)
  }
  return
}
