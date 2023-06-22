copy from github.com/golang/protobuf/jsonpb

chang encode.go, func marshalSingularValue, line 548
    // w.write(fmt.Sprintf(`"%d"`, v.Interface()))
	w.write(fmt.Sprintf(`%d`, v.Interface()))

解决int64类型被转为string类型