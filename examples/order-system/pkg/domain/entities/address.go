package entities

// Address地址值对象
type Address struct {
	Street  string
	City    string
	State   string
	ZipCode string
	Country string
}

// Validate 验证地址有效性
func (a Address) Validate() bool {
	return a.Street != "" && a.City != "" && a.Country != ""
}
