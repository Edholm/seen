package main

import (
    "database/sql"
    "fmt"
    "github.com/edholm/seendb"
    _ "github.com/go-sql-driver/mysql"
    "time"
)

func handleError(err error) {
    if err != nil {
        panic(err.Error())
    }
}

func main() {
    dsn := fmt.Sprintf("%s:%s@(%s)/seen?parseTime=true",
        seendb.User(), seendb.Password(), seendb.Host())
    db, err := sql.Open("mysql", dsn)
    handleError(err)
    defer db.Close()

    rows, err := db.Query("SELECT * FROM History")
    handleError(err)
    defer rows.Close()

    var (
        name    string
        season  int
        episode int
        date    time.Time // we "scan" the result in here
    )

    for rows.Next() {
        err := rows.Scan(&name, &season, &episode, &date)
        handleError(err)

        fmt.Printf("saw %v (S%02dE%02d) at %v\n",
            name, season, episode, date.Format(time.RFC1123))
    }
}
