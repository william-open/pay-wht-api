package system

import (
	"log"
)

var BotChatID string

func Config() {

	BotChatID = (&ConfigSystem{}).GetConfigCacheByConfigKey("sys.telegram.notify.group").ConfigValue

	log.Printf("Telegram, 异常报错群ID: %s", BotChatID)

}
