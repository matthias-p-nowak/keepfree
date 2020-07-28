package main

import(
  "encoding/gob"
  "log"
  "os"
)

func StoreData(fileName string, data interface{}) {
  f,err := os.Create(fileName)
  if err != nil {
    log.Fatal(err.Error())
  }
  defer f.Close()
  enc:=gob.NewEncoder(f)
  err=enc.Encode(data)
  
  if err != nil {
    log.Fatal(err.Error())
  }
}

func RetrieveData(fileName string, data interface{}) (err error){
  f,err:=os.Open(fileName)
  if err != nil {
    return
  }
  defer f.Close()
  dec:=gob.NewDecoder(f)
  err=dec.Decode(data)
  if err != nil {
    log.Fatal(err.Error())
  }
  return
}
