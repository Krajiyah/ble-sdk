package util

import "strings"

const (
	ForcePanicMsgPrefix = "force_panic: "
)

type TryCatchBlock struct {
	Try     func()
	Catch   func(error)
	Finally func()
}

func Throw(up error) {
	panic(up)
}

func (tcf TryCatchBlock) Do() {
	if tcf.Finally != nil {
		defer tcf.Finally()
	}
	if tcf.Catch != nil {
		defer func() {
			if r := recover(); r != nil {
				err, _ := r.(error)
				if strings.HasPrefix(err.Error(), ForcePanicMsgPrefix) {
					panic(err)
				} else {
					tcf.Catch(err)
				}
			}
		}()
	}
	tcf.Try()
}
