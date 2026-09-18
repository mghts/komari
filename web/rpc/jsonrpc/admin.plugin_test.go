package jsonrpc

import (
	"context"
	"os"
	"testing"

	"github.com/komari-monitor/komari/cmd/flags"
	"github.com/komari-monitor/komari/database"
	"github.com/komari-monitor/komari/database/dbcore"
	"github.com/komari-monitor/komari/database/models"
	"github.com/komari-monitor/komari/internal/config"
	"github.com/komari-monitor/komari/pkg/rpc"
	"github.com/komari-monitor/komari/utils/messageSender"
	"github.com/komari-monitor/komari/utils/messageSender/factory"
)

// TestMain retains the shared SQLite setup used by the JSON-RPC test suite.
func TestMain(m *testing.M) {
	flags.DatabaseType = flags.DatabaseTypeSQLite
	flags.DatabaseFile = "file:komari_jsonrpc_test?mode=memory&cache=shared"
	db := dbcore.GetDBInstance()
	if sqlDB, err := db.DB(); err == nil {
		sqlDB.SetMaxOpenConns(1)
	}
	os.Exit(m.Run())
}

func TestRetiredPluginMethodsAreUnavailable(t *testing.T) {
	for _, method := range []string{"admin:listPlugins", "admin:setPluginEnabled", "admin:getPluginLogs", "admin:deletePlugin", "admin:getPluginConfiguration", "admin:setPluginConfiguration"} {
		response := rpc.Call(1, method, map[string]any{})
		if response.Error == nil || response.Error.Code != rpc.MethodNotFound {
			t.Fatalf("%s remains callable: %+v", method, response)
		}
	}
}

func TestRetiredNotificationProviderPreservesSavedData(t *testing.T) {
	const script = `{"script":"function sendMessage() { throw new Error('must never run'); }"}`
	oldMethod, _ := config.GetAs[string](config.NotificationMethodKey, "none")
	t.Cleanup(func() { _ = config.Set(config.NotificationMethodKey, oldMethod); _ = messageSender.Shutdown() })
	if err := database.SaveMessageSenderConfig(&models.MessageSenderProvider{Name: "Javascript", Addition: script}); err != nil {
		t.Fatal(err)
	}
	if err := config.Set(config.NotificationMethodKey, "Javascript"); err != nil {
		t.Fatal(err)
	}
	if err := messageSender.LoadProvider("empty", "{}"); err != nil {
		t.Fatal(err)
	}
	messageSender.Initialize()
	if messageSender.CurrentProvider() != nil {
		t.Fatal("unavailable provider must not retain the previous active sender")
	}
	if _, exists := factory.GetConstructor("Javascript"); exists {
		t.Fatal("retired sender registered")
	}
	saved, err := database.GetMessageSenderConfigByName("Javascript")
	if err != nil || saved.Addition != script {
		t.Fatalf("saved script changed: %v", err)
	}
	method, err := config.GetAs[string](config.NotificationMethodKey)
	if err != nil || method != "Javascript" {
		t.Fatal("legacy selection must remain visible for recovery")
	}
	for _, name := range []string{"getMessageSenderProvider", "setMessageSenderProvider", "editSettings"} {
		params := map[string]any{"provider": "Javascript", "name": "Javascript", "addition": script, "notification_method": "Javascript"}
		resp := rpc.CallWithContext(context.Background(), 1, "admin:"+name, params)
		if resp.Error == nil {
			t.Fatalf("%s accepted retired provider", name)
		}
	}
	if err := messageSender.LoadProvider("empty", "{}"); err != nil {
		t.Fatalf("supported provider stopped working: %v", err)
	}
}
