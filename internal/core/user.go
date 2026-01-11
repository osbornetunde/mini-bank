package core

type User struct {
	ID          int
	Email       string
	FirstName   string
	LastName    string
	Balance     *int64
	Password    *string
	Permissions []string
	Accounts    []*Account
}

func (u *User) CalculateTotalBalance() int64 {
	var total int64
	for _, acc := range u.Accounts {
		total += acc.Balance
	}
	return total
}
