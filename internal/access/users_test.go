package access

import "testing"

func TestPasswordHashPolicy(t *testing.T) {
	password := "correct horse battery staple"
	a, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword(password)
	if err != nil {
		t.Fatal(err)
	}
	if a == b || !VerifyPassword(password, a) || VerifyPassword("another sufficiently long password", a) || VerifyPassword(password, DummyPasswordHash) {
		t.Fatal("password verification or salt failed")
	}
	if VerifyPassword(password, "$argon2id$v=19$m=999999999,t=99,p=99$bad$bad") {
		t.Fatal("unbounded hash parameters accepted")
	}
	if _, err := HashPassword("short"); err == nil {
		t.Fatal("short password accepted")
	}
	if NormalizeUsername(" Alice ") != "alice" || ValidUsername("a") || ValidUsername("alice@example.test") {
		t.Fatal("username policy")
	}
}
func TestLoginWorkAndAttemptBudgets(t *testing.T) {
	var g LoginGuard
	var done []func()
	for i := 0; i < 4; i++ {
		release, ok := g.Acquire("alice", false)
		if !ok {
			t.Fatal("slots unavailable")
		}
		done = append(done, release)
	}
	if _, ok := g.Acquire("bob", false); ok {
		t.Fatal("unbounded KDF work")
	}
	for _, release := range done {
		release()
	}
	for i := 0; i < 10; i++ {
		release, ok := g.Acquire("alice", true)
		if !ok {
			t.Fatal("premature rate limit")
		}
		release()
	}
	if _, ok := g.Acquire("alice", true); ok {
		t.Fatal("account budget not enforced")
	}
}
