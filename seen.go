package main

import (
    "database/sql"
    "fmt"
    "github.com/edholm/seendb"
    _ "github.com/lib/pq"
    "github.com/spf13/cobra"
    "log"
    "os"
    "time"
)

// Term colors
const (
    Reset = "\x1b[0m"

    FgBlack   = "\x1b[30m"
    FgRed     = "\x1b[31m"
    FgGreen   = "\x1b[32m"
    FgYellow  = "\x1b[33m"
    FgBlue    = "\x1b[34m"
    FgMagenta = "\x1b[35m"
    FgCyan    = "\x1b[36m"
    FgWhite   = "\x1b[37m"
)

// Flag related variables
var (
    historyCount     int
    isAnime, isScene bool
    vlog             VerboseLog
    db               *sql.DB
)

// We scan the SQL results into these variables
var (
    name    string
    season  int
    episode int
    added   time.Time
)

type VerboseLog struct {
    verbose bool
    log     *log.Logger
}

func (vl VerboseLog) Println(v ...interface{}) {
    if vl.verbose {
        log.Println(v...)
    }
}

func handleError(err error) {
    if err != nil {
        panic(err.Error())
    }
}

func history(cmd *cobra.Command, args []string) {
    vlog.Println("Fetching the", historyCount, " last entries...")

    rows, err := db.Query("SELECT * FROM History ORDER BY added DESC LIMIT $1", historyCount)
    handleError(err)
    defer rows.Close()

    vlog.Println("Printing found rows...")
    for rows.Next() {
        err := rows.Scan(&name, &season, &episode, &added)
        handleError(err)

        fmt.Printf("saw %v (S%02dE%02d) at %v\n",
            name, season, episode, added.Format(time.RFC1123))
    }
}

func add(cmd *cobra.Command, args []string) {
    vlog.Println("Preparing to add  ", len(args), "new shows")
}

func record(cmd *cobra.Command, args []string) {
    vlog.Println("Appending history...")
}

func exists(cmd *cobra.Command, args []string) {
    if len(args) == 0 {
        log.Println("[Exists] You must supply a name")
        return
    } else if len(args) > 1 {
        vlog.Println("Skipping existence check for", len(args)-1, "names. Only checking", args[0])
    }

    sql := "SELECT COUNT(1) FROM shows WHERE name=$1"
    var count int
    err := db.QueryRow(sql, args[0]).Scan(&count)
    handleError(err)

    if count > 0 {
        fmt.Println(args[0], FgGreen+"exists", Reset)
    } else {
        fmt.Println(args[0], "does "+FgRed+"NOT"+Reset+" exist")
    }
}

func search(cmd *cobra.Command, args []string) {
    vlog.Println("Preparing search...")
    if len(args) == 0 {
        log.Println("[Search] You must supply a name")
        return
    } else if len(args) > 1 {
        vlog.Println("Skipping", len(args)-1, "arguments")
    }

    //FIXME
    sql := "SELECT name FROM shows WHERE name = $1 LIMIT 1"
    err := db.QueryRow(sql, args[0]).Scan(&name)
    handleError(err)

    fmt.Println(name)
}

func initCobra() {
    var cmdHistory = &cobra.Command{
        Use:   "history [regex-query]",
        Short: "Print the history",
        Long: `Print the history only matching the supplied regex.
by default it prints the last 15 shows`,
        Run: history,
    }
    cmdHistory.Flags().IntVarP(&historyCount, "count", "c", 15, "How many history lines to print")

    var cmdRecord = &cobra.Command{
        Use:   "record [name] [season] [episode]",
        Short: "Add the supplied show to history",
        Long: `If --is-scene or --is-anime is set then only name need
be supplied. Then the season and episode is parsed from the file name.`,
        Run: record,
    }
    cmdRecord.Flags().BoolVarP(&isScene, "is-scene", "s", false, "Whether or not the supplied tv-show is a scene file")
    cmdRecord.Flags().BoolVarP(&isAnime, "is-anime", "a", false, "Whether or not the supplied tv-show is a anime show")

    var cmdAdd = &cobra.Command{
        Use:   "add [name]...",
        Short: "Add new shows to the database",
        Long: `Add new shows to the database for later use in history.
Note that a show first needs to be added before it can be used in "seen history"`,
        Run: add,
    }

    var cmdExists = &cobra.Command{
        Use:   "exists [name]",
        Short: "Print existence of supplied show",
        Run:   exists,
    }

    var cmdSearch = &cobra.Command{
        Use:   "search [name]",
        Short: "Search for existence of the supplied show",
        Run:   search,
    }
    var rootCmd = &cobra.Command{Use: "seen"}
    rootCmd.AddCommand(cmdHistory, cmdRecord, cmdAdd, cmdExists, cmdSearch)
    rootCmd.PersistentFlags().BoolVarP(&vlog.verbose, "verbose", "v", false, "Show what is happening")
    rootCmd.Execute()
}

func main() {
    vlog.log = log.New(os.Stdout, "", log.Ldate)

    connectLine := fmt.Sprintf("user=%s password=%s host=%s dbname=seen sslmode=require",
        seendb.User(), seendb.Password(), seendb.Host())

    var err error
    db, err = sql.Open("postgres", connectLine)
    handleError(err)
    defer db.Close()

    initCobra()
}
