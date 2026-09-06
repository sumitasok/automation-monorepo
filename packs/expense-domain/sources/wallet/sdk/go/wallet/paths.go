package wallet

import "strconv"

// itemPath renders the dotted path of one batch item, for error messages.
func itemPath(i int) string { return "items[" + strconv.Itoa(i) + "]" }

// recordPath renders the dotted path of one create-record item.
func recordPath(i int) string { return "records[" + strconv.Itoa(i) + "]" }
