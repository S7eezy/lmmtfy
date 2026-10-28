package expr

// Doc describes a function for people writing formulas: editors show it
// while they type.
type Doc struct {
	Name    string `json:"name"`
	Args    string `json:"args"` // "(text, length)"
	Summary string `json:"summary"`
	Group   string `json:"group"`
}

// Docs lists every function, by group then in a sensible reading order.
// TestDocsCoverFunctions keeps it in sync with the functions themselves.
func Docs() []Doc { return docs }

var docs = []Doc{
	{"if", "(condition, then, else)", "then when the condition is true, else otherwise (empty without else)", "Logic"},
	{"ifs", "(condition1, result1, condition2, result2, …, otherwise)", "the result of the first true condition", "Logic"},
	{"coalesce", "(a, b, …)", "the first value that is not empty", "Logic"},
	{"is_empty", "(value)", "true when the value is empty", "Logic"},

	{"round", "(number, decimals)", "rounded half away from zero; without decimals, a whole number", "Numbers"},
	{"floor", "(number)", "the whole number below", "Numbers"},
	{"ceil", "(number)", "the whole number above", "Numbers"},
	{"abs", "(number)", "the number without its sign", "Numbers"},
	{"min", "(a, b, …)", "the smallest value, ignoring empty ones", "Numbers"},
	{"max", "(a, b, …)", "the largest value, ignoring empty ones", "Numbers"},
	{"number", "(text)", "reads text like 1234.5 as a number", "Numbers"},

	{"upper", "(text)", "in upper case", "Text"},
	{"lower", "(text)", "in lower case", "Text"},
	{"trim", "(text)", "without spaces at both ends", "Text"},
	{"len", "(text)", "the number of characters", "Text"},
	{"left", "(text, length)", "the first characters", "Text"},
	{"right", "(text, length)", "the last characters", "Text"},
	{"mid", "(text, start, length)", "characters from start (1 = first)", "Text"},
	{"replace", "(text, find, with)", "every find replaced", "Text"},
	{"pad_left", "(text, length, character)", "padded on the left to length, e.g. pad_left(text([id]), 6, \"0\")", "Text"},
	{"split_part", "(text, separator, n)", "the nth part between separators (1 = first)", "Text"},
	{"text", "(value)", "any value as text: 1234.5, 2024-01-31, true", "Text"},

	{"date", "(\"YYYY-MM-DD\")", "a date", "Dates"},
	{"datetime", "(\"YYYY-MM-DDTHH:mm:ss\")", "a datetime; with an offset (Z, +02:00) or in UTC", "Dates"},
	{"add_days", "(date, days)", "days later (negative: earlier)", "Dates"},
	{"add_months", "(date, months)", "months later; Jan 31 + 1 month is the end of February", "Dates"},
	{"days_between", "(from, to)", "the number of days from one date to the other", "Dates"},
	{"year", "(date)", "the year", "Dates"},
	{"month", "(date)", "the month, 1 to 12", "Dates"},
	{"day", "(date)", "the day of the month", "Dates"},
	{"hour", "(datetime)", "the hour, 0 to 23", "Dates"},
	{"minute", "(datetime)", "the minutes", "Dates"},
	{"from_epoch", "(number, \"s\" | \"ms\" | \"us\")", "the datetime of a Unix timestamp", "Dates"},
	{"to_epoch", "(datetime, \"s\" | \"ms\" | \"us\")", "the Unix timestamp of a datetime", "Dates"},
	{"today", "()", "the date of the conversion (changes from run to run)", "Dates"},
	{"now", "()", "the time of the conversion (changes from run to run)", "Dates"},

	{"row", "()", "the row number in the input file (1 = first data row)", "Rows"},

	{"first", "(value)", "the first value that is not empty, over the whole file (or batch)", "Aggregates"},
	{"last", "(value)", "the last value that is not empty, over the whole file (or batch)", "Aggregates"},
	{"lowest", "(value)", "the smallest value (earliest date) over the whole file (or batch)", "Aggregates"},
	{"highest", "(value)", "the largest value (latest date) over the whole file (or batch)", "Aggregates"},
	{"total", "(number)", "the sum over the whole file (or batch), e.g. total([price] * [qty])", "Aggregates"},
	{"mean", "(number)", "the average over the whole file (or batch)", "Aggregates"},
	{"count", "(value)", "the number of values that are not empty; count() counts rows", "Aggregates"},
}
