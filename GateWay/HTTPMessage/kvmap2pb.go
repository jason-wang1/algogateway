package HTTPMessage

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"
	protoV2 "google.golang.org/protobuf/proto"
)

func switch_param_must_exist(name string) bool {
	switch name {
	case "request_id":
		return true
	case "user_id":
		return true
	case "ll_id":
		return true
	case "res_type":
		return true
	default:
		return false
	}
}

// kvMap2pb 将kvMap(map[string][string])转为Proto
func kvMap2pb(
	kvMap map[string]string,
	protoMessage protoV2.Message,
) ([]byte, error) {
	// 获取结构体实例的反射类型对象, 遍历结构体成员
	typeOfRequest := reflect.TypeOf(protoMessage).Elem()
	valueOfRequest := reflect.ValueOf(protoMessage).Elem()
	for idx := 0; idx < valueOfRequest.NumField(); idx++ {
		// 非导出字段 或 无法取址则不能Set
		value := valueOfRequest.Field(idx)
		if !value.CanSet() {
			continue
		}

		// 从标签(也就是``里面的内容)中获取json字段
		field := typeOfRequest.Field(idx)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" {
			return nil, errors.New("proto value json tag empty, idx = " + strconv.Itoa(idx))
		}

		// 再根据json字段拆分得到名字
		name := strings.Split(jsonTag, ",")[0]
		if name == "" {
			return nil, errors.New("proto value name empty, idx = " + strconv.Itoa(idx))
		}

		if name == "request_id" {
			// uuid 需要从 requestid 来获取
			if value.Kind() == reflect.String {
				str_uuid, ok := kvMap["request_id"]
				if ok && str_uuid != "" {
					value.SetString(str_uuid)
				} else {
					// 如果读取uuid失败或为空, 则自动赋予一个带有auto前缀的uuid值
					value.SetString("auto_" + uuid.New().String())
				}
			} else {
				return nil, errors.New("proto uuid type is not string.")
			}
		} else {
			// 没有特殊转换的proto参数, 采用通配规则, 根据名称获取参数
			param, ok := kvMap[name]
			if !ok {
				// 如果获取不到, 则判断参数是否必须存在
				// 必须存在则报错, 非必须存在则跳过
				if switch_param_must_exist(name) {
					return nil, errors.New("params " + name + " is not exist.")
				} else {
					continue
				}
			}

			switch value.Kind() {
			case reflect.String:
				if utf8.ValidString(param) {
					value.SetString(param)
				} else if switch_param_must_exist(name) {
					return nil, errors.New("params " + name + " is not invalid UTF-8")
				} else {
					continue
				}
			case reflect.Int32:
				if int32Val, err := strconv.ParseInt(param, 10, 32); err == nil {
					value.SetInt(int32Val)
				} else if switch_param_must_exist(name) {
					return nil, err
				} else {
					continue
				}
			case reflect.Int64:
				if int64Val, err := strconv.ParseInt(param, 10, 64); err == nil {
					value.SetInt(int64Val)
				} else if switch_param_must_exist(name) {
					return nil, err
				} else {
					continue
				}
			case reflect.Bool:
				if boolVal, err := strconv.ParseBool(param); err == nil {
					value.SetBool(boolVal)
				} else if switch_param_must_exist(name) {
					return nil, err
				} else {
					continue
				}
			case reflect.Slice:
				// 针对切片类型, 需要按json的Array来解析
				tirmString := strings.Trim(param, "[ ]")
				if tirmString == "" {
					continue
				}

				sliceString := strings.Split(tirmString, ",")
				if value.Type().String() == "[]int32" {
					for sliceIdx := 0; sliceIdx < len(sliceString); sliceIdx++ {
						// P.s> 这里val获取出来的是32位的int64类型的值, 需要将int64强转int32
						val, err := strconv.ParseInt(strings.Trim(sliceString[sliceIdx], " "), 10, 32)
						if err == nil {
							value.Set(reflect.Append(value, reflect.ValueOf(int32(val))))
						} else if switch_param_must_exist(name) {
							return nil, err
						} else {
							continue
						}
					}
				} else if value.Type().String() == "[]int64" {
					for sliceIdx := 0; sliceIdx < len(sliceString); sliceIdx++ {
						int64Val, err := strconv.ParseInt(strings.Trim(sliceString[sliceIdx], " "), 10, 64)
						if err == nil {
							value.Set(reflect.Append(value, reflect.ValueOf(int64Val)))
						} else if switch_param_must_exist(name) {
							return nil, err
						} else {
							continue
						}
					}
				} else if value.Type().String() == "[]string" {
					for sliceIdx := 0; sliceIdx < len(sliceString); sliceIdx++ {
						strVal := strings.Trim(sliceString[sliceIdx], " ")
						value.Set(reflect.Append(value, reflect.ValueOf(strVal)))
					}
				} else {
					return nil, errors.New("params " + name + " proto Slice type is not support, please check gateway code.")
				}
			default:
				return nil, errors.New("params " + name + " proto type is not support, please check gateway code.")
			}
		}
	}

	// proto 转 []byte
	request, err := protoV2.Marshal(protoMessage)
	if err != nil {
		return nil, err
	}
	return request, nil
}
