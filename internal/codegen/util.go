package codegen

func Find[E any](coll []E, f func(E) bool) *E {
	for _, each := range coll {
		if f(each) {
			return &each
		}
	}
	return nil
}
