package i18n

// Message is a translatable text. Other is required; One is used for n == 1
// when formatting plurals.
type Message struct {
	One, Other string
}

// Catalog maps keys to the messages of one language.
type Catalog map[Key]Message

// catalogs registers every supported language; one catalog_xx.go per entry.
var catalogs = map[Lang]Catalog{
	EN: catalogEN,
	DE: catalogDE,
}

func msg(s string) Message             { return Message{Other: s} }
func plural(one, other string) Message { return Message{One: one, Other: other} }
