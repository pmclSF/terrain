package prtemplates

import "sort"

func sortTemplates(s []Template) {
	sort.Slice(s, func(i, j int) bool { return s[i].SignalType < s[j].SignalType })
}

func sortStrings(s []string) {
	sort.Strings(s)
}
