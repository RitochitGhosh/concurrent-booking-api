package auth

import "testing"

func TestPasswordHash(t *testing.T) {
	hash, err := hashPassword("a-long-test-password")
	if err != nil {
		t.Fatal(err)
	}
	if !checkPassword(hash, "a-long-test-password") {
		t.Fatal("valid password rejected")
	}
	if checkPassword(hash, "wrong-password") || checkPassword("invalid", "a-long-test-password") {
		t.Fatal("invalid password accepted")
	}
}
func TestTokenParsing(t *testing.T) {
	for _, value := range []string{"", "x.y", "../../x", "00000000-0000-0000-0000-000000000000.invalid"} {
		if _, _, err := tokenParts(value); err == nil {
			t.Errorf("accepted %q", value)
		}
	}
}
