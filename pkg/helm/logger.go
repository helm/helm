package helm

import (
	"fmt"
	rls "k8s.io/helm/pkg/proto/hapi/services"
)

type TillerOutputLogger interface {
	Log(level rls.LogItem_Level, message string)
}

type ConsoleLogger struct {}

func (logger *ConsoleLogger) Log(level rls.LogItem_Level, message string)  {
	switch level {
		case rls.LogItem_INFO:
			fmt.Println(message)
			break
		case rls.LogItem_ERROR:
			fmt.Println(fmt.Sprintf("Error: %s", message))
			break
	}
}
