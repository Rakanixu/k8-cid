go run main.go -config juno=mercury,cerberus,venus -config vulcan=kronos -config public=mongodb,rabbitmq -config gateway=ambassador
go run main.go -repos juno=089eb18d -repos vulcan=9d80182c -repos public=latest -repos gateway=0.31.0 create
go run main.go -repos juno=089eb18d -repos vulcan=9d80182c -repos public=latest -repos gateway=0.31.0 delete