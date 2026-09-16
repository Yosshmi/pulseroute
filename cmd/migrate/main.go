package main
import("context";"log";"os";"time";"pulseroute/internal/database";"pulseroute/migrations")
func main(){ctx,cancel:=context.WithTimeout(context.Background(),time.Minute);defer cancel();p,err:=database.Open(ctx,os.Getenv("DATABASE_URL"));if err!=nil{log.Fatal(err)};defer p.Close();if err=migrations.Apply(ctx,p);err!=nil{log.Fatal(err)};log.Print("migrations applied")}
