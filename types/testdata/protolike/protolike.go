package protolike

type Message struct {
	state         messageState
	Id            *string
	Child         *Child
	unknownFields unknownFields
	sizeCache     sizeCache
}

func (m *Message) GetId() string {
	if m != nil && m.Id != nil {
		return *m.Id
	}
	return ""
}

func (m *Message) GetChild() *Child {
	if m != nil {
		return m.Child
	}
	return nil
}

func (m *Message) ProtoReflect() ReflectMessage {
	return m.state.message
}

type Child struct {
	Name string
}

type messageState struct {
	message ReflectMessage
}

type unknownFields struct {
	fields map[FieldDescriptor]Value
}

type sizeCache int

type Name string

type Value interface {
	Descriptor() FieldDescriptor
	Message() ReflectMessage
}

type ReflectMessage interface {
	Descriptor() MessageDescriptor
	Range(func(FieldDescriptor, Value) bool)
	New() ReflectMessage
}

type MessageDescriptor interface {
	ParentFile() FileDescriptor
	Fields() FieldDescriptors
	Message() MessageDescriptor
}

type FileDescriptor interface {
	Messages() MessageDescriptors
	Imports() FileImports
}

type FieldDescriptor interface {
	Parent() MessageDescriptor
	Message() MessageDescriptor
	Enum() EnumDescriptor
}

type EnumDescriptor interface {
	ParentFile() FileDescriptor
	Values() EnumValueDescriptors
}

type FieldDescriptors interface {
	Get(int) FieldDescriptor
	ByName(Name) FieldDescriptor
}

type MessageDescriptors interface {
	Get(int) MessageDescriptor
	ByName(Name) MessageDescriptor
}

type EnumValueDescriptors interface {
	Get(int) EnumDescriptor
	ByName(Name) EnumDescriptor
}

type FileImports interface {
	Get(int) FileDescriptor
	ByPath(string) FileDescriptor
}
