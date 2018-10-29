go run main.go -config juno=mercury,venus -config vulcan=kronos -config public=mongodb,rabbitmq,ambassador
go run main.go -repos juno=aaaa1 -repos vulcan=bbbb1 -repos public=latest create
go run main.go -repos juno=aaaa1 -repos vulcan=bbbb1 -repos public=latest delete