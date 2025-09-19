package system

var BotChatID string

func Config() {

	BotChatID = (&ConfigSystem{}).GetConfigCacheByConfigKey("sys.telegram.notify.group").ConfigValue

}
