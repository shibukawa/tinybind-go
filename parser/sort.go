package parser

import "sort"

func sortStrings(ss []string) {
	sort.Strings(ss)
}

func sortRoutes(routes []Route) {
	sort.SliceStable(routes, func(i, j int) bool {
		a, b := routes[i], routes[j]
		if a.Method != b.Method {
			return a.Method < b.Method
		}
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		if a.Handler.Name != b.Handler.Name {
			return a.Handler.Name < b.Handler.Name
		}
		if a.Handler.Form != b.Handler.Form {
			return a.Handler.Form < b.Handler.Form
		}
		// Two registrations of one pattern by one handler differ only by site.
		if a.Site.File != b.Site.File {
			return a.Site.File < b.Site.File
		}
		if a.Site.Line != b.Site.Line {
			return a.Site.Line < b.Site.Line
		}
		return a.Site.Column < b.Site.Column
	})
}
