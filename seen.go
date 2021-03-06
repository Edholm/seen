package main

import (
    "database/sql"
    "fmt"
    "github.com/dustin/go-humanize"
    "github.com/edholm/seendb"
    _ "github.com/lib/pq"
    "github.com/spf13/cobra"
    "log"
    "os"
    "strconv"
    "time"
)

const currentVersion = "0.9 Beta"

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
    delay        string
    historyCount int
    anime, scene string
    shortFormat  bool
    onlyEpisode  bool
    vlog         VerboseLog
    db           *sql.DB
)

// We scan the SQL results into these variables
type Show struct {
    name    string
    season  int
    episode int
    added   time.Time
}

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
    vlog.Println("Fetching the", historyCount, "last entries...")
    sqlBeg := "SELECT * FROM History"
    sqlEnd := " ORDER BY added DESC LIMIT $1"

    var rows *sql.Rows
    var err error

    al := len(args)
    switch {
    case al == 1:
        sqlBeg += " WHERE name=$2"
        rows, err = db.Query(sqlBeg+sqlEnd, historyCount, args[0])
    case al == 2:
        sqlBeg += " WHERE name=$2 AND season=$3"
        rows, err = db.Query(sqlBeg+sqlEnd, historyCount, args[0], args[1])
    case al == 3:
        sqlBeg += " WHERE name=$2 AND season=$3 AND episode=$4"
        rows, err = db.Query(sqlBeg+sqlEnd, historyCount, args[0], args[1], args[2])
    default:
        rows, err = db.Query(sqlBeg+sqlEnd, historyCount)
    }
    handleError(err)
    defer rows.Close()

    var show Show
    vlog.Println("Printing found rows...")
    for rows.Next() {
        err := rows.Scan(&show.name, &show.season, &show.episode, &show.added)
        handleError(err)

        fmt.Printf("saw %s%-20v%s %s(%sS%s%02d%sE%s%02d%s%s)%s %v\n",
            FgMagenta, show.name, Reset, FgBlue, Reset, FgCyan, show.season, Reset, FgGreen,
            show.episode, Reset, FgBlue, Reset, timeString(show.added))
    }
}

func add(cmd *cobra.Command, args []string) {
    vlog.Println("Preparing to add", len(args), "new shows")

    sql := "INSERT INTO shows(name) VALUES($1)"
    for _, value := range args {
        if !showExists(value) {
            vlog.Println("Adding", "\""+value+"\"")
            _, err := db.Exec(sql, value)
            handleError(err)

        } else {
            vlog.Println("\""+value+"\"", "already exists. Skipping...")
        }
    }
}

func version(cmd *cobra.Command, args []string) {
    fmt.Println("Version:", currentVersion)
    os.Exit(0)
}

func delayToTimestamp(d string) time.Time {
    duration, err := time.ParseDuration(d)
    if err != nil {
        handleError(err)
    }

    return time.Now().Add(duration)
}

func record(cmd *cobra.Command, args []string) {
    vlog.Println("Appending history...")
    if len(args)%3 != 0 {
        log.Println("Wrong number of arguments supplied. Supplied:", len(args), "Need 3")
    }

    var name string
    sql := "INSERT INTO history(name, season, episode, added) VALUES($1, $2, $3, $4)"
    for i := 0; i < len(args); i += 3 {
        name = args[i]
        season, err := strconv.Atoi(args[i+1])
        episode, err2 := strconv.Atoi(args[i+2])

        if err != nil || err2 != nil {
            log.Println("Unable to parse season or episode for", "\""+args[i]+"\".", "Season:", season, "Episode:", episode, "Skipping...")
            continue
        }

        _, err3 := db.Exec(sql, name, season, episode, delayToTimestamp(delay))
        if err3 != nil {
            log.Println(FgRed+"Unable to add", "\""+name+"\"", "to the database. Msg: ", err3.Error(), Reset)
        }
    }
}

func listShows(cmd *cobra.Command, args []string) {
    vlog.Println("Preparing to list all shows")
    if len(args) > 0 {
        vlog.Println("Ignoring", len(args), "argument(s)")
    }

    sql := "SELECT name, added FROM shows ORDER BY added DESC"
    rows, err := db.Query(sql)
    handleError(err)

    var (
        name  string
        added time.Time
        count int
    )
    for rows.Next() {
        err := rows.Scan(&name, &added)
        handleError(err)

        count++
        if shortFormat {
            fmt.Println(name)
        } else {
            fmt.Printf("%s%-30s%s added %v\n", FgMagenta, name, Reset, timeString(added))
        }
    }
    vlog.Println(count, "shows listed")
}

func printNext(cmd *cobra.Command, args []string) {
    for _, name := range args {
        show, err := getShow(name)
        if err == sql.ErrNoRows {
            log.Println("Couldn't find", name, "in database")
            continue
        } else if err != nil {
            handleError(err)
        }
        if !onlyEpisode {
            fmt.Printf("%s: S%02dE%02d\n", show.name, show.season, show.episode+1)
        } else {
            fmt.Println(show.episode + 1)
        }
    }
}

// Humanized or verbose time to string converter
func timeString(t time.Time) (str string) {
    if vlog.verbose {
        str = t.Format(time.RFC1123Z)
    } else {
        str = humanize.Time(t)
    }
    return
}

func getShow(name string) (show Show, err error) {
    sqlStmt := "SELECT * FROM history WHERE name = $1 ORDER BY added DESC LIMIT 1"
    err = db.QueryRow(sqlStmt, name).Scan(&show.name, &show.season, &show.episode, &show.added)
    return
}

// Query the db on whether or not the show exists
func showExists(name string) bool {
    sql := "SELECT COUNT(1) FROM shows WHERE name=$1"
    var count int
    err := db.QueryRow(sql, name).Scan(&count)
    handleError(err)

    return count > 0
}

func initCobra() {
    var cmdHistory = &cobra.Command{
        Use:   "history [show] [season] [episode]",
        Short: "Print the history",
        Long: `Print the history only matching the supplied filter.
by default it prints the last 5 history items`,
        Run: history,
    }
    cmdHistory.Flags().IntVarP(&historyCount, "count", "c", 5, "How many history lines to print")

    var cmdRecord = &cobra.Command{
        Use:   "record [name] [season] [episode]",
        Short: "Add the supplied show to history",
        Long: `If --scene or --anime is set then any arguments supplied
are ignored. The name, season and episode is parsed from the file name supplied.`,
        Run: record,
    }
    cmdRecord.Flags().StringVarP(&delay, "delay", "d", "0s", "Delay recording by this much. Accepts 10s, 10m, 10h")
    //cmdRecord.Flags().StringVarP(&scene, "scene", "s", "", "Scene based file name to parse")
    //cmdRecord.Flags().StringVarP(&anime, "anime", "a", "", "Anime file name to parse")

    var cmdAdd = &cobra.Command{
        Use:   "add [name]...",
        Short: "Add new shows to the database",
        Long: `Add new shows to the database for later use in history.
Note that a show first needs to be added before it can be used in "seen history"`,
        Run: add,
    }

    var cmdShows = &cobra.Command{
        Use:   "shows",
        Short: "List all shows added",
        Run:   listShows,
    }
    cmdShows.Flags().BoolVarP(&shortFormat, "short-format", "s", false, "List shows with short format.")

    var cmdNext = &cobra.Command{
        Use:   "next [name]",
        Short: "Print the next episode. E.g. Macgyver: S01E02",
        Run:   printNext,
    }
    cmdNext.Flags().BoolVarP(&onlyEpisode, "episode", "e", false, "List only the next episode(s).")

    var cmdVersion = &cobra.Command{Use: "version", Run: version}

    var rootCmd = &cobra.Command{Use: "seen"}
    rootCmd.AddCommand(cmdHistory, cmdRecord, cmdAdd, cmdShows, cmdVersion, cmdNext)
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
