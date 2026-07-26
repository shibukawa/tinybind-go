package generator

// PackageLoadCount reports how many packages have been type-checked so far. It
// exists so tests can assert that one generation run type-checks once.
func PackageLoadCount() int64 { return packageLoadCount.Load() }
