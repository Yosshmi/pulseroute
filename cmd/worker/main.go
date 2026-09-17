package main
import("log";"pulseroute/internal/app")
func main(){if e:=app.Run("worker");e!=nil{log.Fatal(e)}}
