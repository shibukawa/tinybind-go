package generator

// PackageLoadCount reports how many packages have been type-checked so far. It
// exists so tests can assert that one generation run type-checks once.
func PackageLoadCount() int64 { return packageLoadCount.Load() }

// RouteParseCount reports how many route parses have run so far, so tests can
// assert that one generation run parses routes at most once.
func RouteParseCount() int64 { return routeParseCount.Load() }
