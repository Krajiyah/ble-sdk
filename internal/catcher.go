package internal

// Block represents struct for try-catch-finally control flow
type Block struct {
	Try     func()
	Catch   func(error)
	Finally func()
}

// Do executes Block try-catch-finally control flow
func (tcf Block) Do() {
	if tcf.Finally != nil {
		defer tcf.Finally()
	}
	if tcf.Catch != nil {
		defer func() {
			if r := recover(); r != nil {
				err, _ := r.(error)
				tcf.Catch(err)
			}
		}()
	}
	tcf.Try()
}
