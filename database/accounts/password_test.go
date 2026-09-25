package accounts

import (
	"strings"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
)

func TestPasswordHashesAreSaltedAndLegacyHashesRemainRecognized(t *testing.T) {
	first, err := generatePasswordHash("example-password")
	if err != nil {
		t.Fatal(err)
	}
	second, err := generatePasswordHash("example-password")
	if err != nil {
		t.Fatal(err)
	}
	if first == second || !strings.HasPrefix(first, "$argon2id$") {
		t.Fatalf("new password hashes are not independently salted: %q, %q", first, second)
	}
	if !verifyPasswordHash("example-password", first) || verifyPasswordHash("wrong-password", first) {
		t.Fatal("argon2id password verification failed")
	}
	if !verifyPasswordHash("example-password", second) || isArgon2idHash(hashPasswd("example-password")) {
		t.Fatal("password hash format detection failed")
	}
}

func TestLegacyPasswordUpgradesOnSuccessfulLogin(t *testing.T) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:accounts_password_upgrade?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	legacy := models.User{UUID: "legacy-password-test", Username: "legacy-password-test", Passwd: hashPasswd("correct-password")}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	if uuid, ok := CheckPassword(legacy.Username, "wrong-password"); ok || uuid != "" {
		t.Fatal("incorrect legacy password was accepted")
	}
	if uuid, ok := CheckPassword(legacy.Username, "correct-password"); !ok || uuid != legacy.UUID {
		t.Fatal("correct legacy password was rejected")
	}
	var updated models.User
	if err := db.First(&updated, "uuid = ?", legacy.UUID).Error; err != nil {
		t.Fatal(err)
	}
	if !isArgon2idHash(updated.Passwd) || !verifyPasswordHash("correct-password", updated.Passwd) {
		t.Fatal("legacy password was not upgraded to Argon2id")
	}
}
