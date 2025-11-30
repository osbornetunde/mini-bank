package core

type User struct {
	ID        int
	Email     string
	FirstName string
	LastName  string
	Balance   *int
	Password  *string
}
