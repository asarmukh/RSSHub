package helper

import "fmt"

func PrintHelp() {
	fmt.Print(`Usage:
  rsshub COMMAND [OPTIONS]

Commands:
   add             add new RSS feed (--name, --url)
   list            list available RSS feeds [--num N]
   delete          delete RSS feed (--name)
   articles        show latest articles (--feed-name, --num)
   fetch           start background fetching
   set-interval    set RSS fetch interval (--duration 2m)
   set-workers     set number of workers (--count N)
   help            show this help
`)
}
