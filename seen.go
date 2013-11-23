package main

import (
    "database/sql"
    "fmt"
    "github.com/edholm/seendb"
    _ "github.com/lib/pq"
    "github.com/spf13/cobra"
    "log"
    "os"
    "strconv"
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
    historyCount int
    anime, scene string
    shortFormat  bool
    vlog         VerboseLog
    db           *sql.DB
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

func record(cmd *cobra.Command, args []string) {
    vlog.Println("Appending history...")
    if len(args)%3 != 0 {
        log.Println("Wrong number of arguments supplied. Supplied:", len(args), "Need 3")
    }

    sql := "INSERT INTO history(name, season, episode) VALUES($1, $2, $3)"
    for i := 0; i < len(args); i += 3 {
        name = args[i]
        season, err := strconv.Atoi(args[i+1])
        episode, err2 := strconv.Atoi(args[i+2])

        if err != nil || err2 != nil {
            log.Println("Unable to parse season or episode for", "\""+args[i]+"\".", "Season:", args[i+1], "Episode:", args[i+2]+".", "Skipping...")
            continue
        }

        _, err3 := db.Exec(sql, name, season, episode)
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

    var count int
    for rows.Next() {
        err := rows.Scan(&name, &added)
        handleError(err)

        count++
        if shortFormat {
            fmt.Println(name)
        } else {
            fmt.Println(name, "added at", added.Format(time.RFC1123))
        }
    }
}

// the cli exists command
func exists(cmd *cobra.Command, args []string) {
    if len(args) == 0 {
        log.Println("[Exists] You must supply a name")
        return
    } else if len(args) > 1 {
        vlog.Println("Skipping existence check for", len(args)-1, "names. Only checking", args[0])
    }

    if showExists(args[0]) {
        fmt.Println(args[0], FgGreen+"exists", Reset)
    } else {
        fmt.Println(args[0], "does "+FgRed+"NOT"+Reset+" exist")
    }
}

// Query the db on whether or not the show exists
func showExists(name string) bool {
    sql := "SELECT COUNT(1) FROM shows WHERE name=$1"
    var count int
    err := db.QueryRow(sql, name).Scan(&count)
    handleError(err)

    return count > 0
}

func search(cmd *cobra.Command, args []string) {
    vlog.Println("Preparing search...")
    if len(args) == 0 {
        log.Println("[Search] You must supply a name")
        return
    } else if len(args) > 1 {
        vlog.Println("Skipping", len(args)-1, "arguments")
    }

    sql := "SELECT name FROM shows WHERE name ~* $1 LIMIT 1"
    err := db.QueryRow(sql, args[0]).Scan(&name)
    handleError(err)

    fmt.Println(name)
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
    //cmdRecord.Flags().StringVarP(&scene, "scene", "s", "", "Scene based file name to parse")
    //cmdRecord.Flags().StringVarP(&anime, "anime", "a", "", "Anime file name to parse")

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

    var cmdShows = &cobra.Command{
        Use:   "shows",
        Short: "List all shows added",
        Run:   listShows,
    }
    cmdShows.Flags().BoolVarP(&shortFormat, "short-format", "s", false, "List shows with short format.")

    var rootCmd = &cobra.Command{Use: "seen"}
    rootCmd.AddCommand(cmdHistory, cmdRecord, cmdAdd, cmdExists, cmdSearch, cmdShows)
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
