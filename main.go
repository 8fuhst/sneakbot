package main

import (
	"github.com/go-telegram-bot-api/telegram-bot-api"
	"gopkg.in/yaml.v2"
	"log"
	"net/http"
	"os"
	"sneakbot/database"
	"sneakbot/texts"
	"strings"
)

type Config struct {
	Webhook struct {
		Url      string `yaml:"url"`
		ApiToken string `yaml:"apikey"`
	} `yaml:"webhook"`
}

var cfg Config
var bot *tgbotapi.BotAPI

func init() {
	f, err := os.Open("config.yml")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}()

	decoder := yaml.NewDecoder(f)
	err = decoder.Decode(&cfg)
	if err != nil {
		panic(err)
	}
}

func handleMessage(update tgbotapi.Update) {
	msgtext := update.Message.Text
	var err error
	if strings.HasPrefix(msgtext, "/") {
		if strings.HasPrefix(msgtext, "/start") {
			err = handleCommandStart(update)
		} else if strings.HasPrefix(msgtext, "/reset") {
			err = handleCommandReset(update)
		} else if strings.HasPrefix(msgtext, "/draw") {
			err = handleCommandDraw(update)
		} else if strings.HasPrefix(msgtext, "/stop") {
			err = handleCommandStop(update)
		}
	}
	if err != nil {
		log.Fatal(err)
	}
}

func handleCommandStart(update tgbotapi.Update) error {
	err := sendPoll(update, texts.Start_message)
	return err
}

func handleCommandReset(update tgbotapi.Update) error {
	database.ResetGroup(update.Message.Chat.ID)
	err := sendPoll(update, texts.Reset_message)
	return err
}

func handleCommandDraw(update tgbotapi.Update) error {
	answer := tgbotapi.NewMessage(update.Message.Chat.ID, texts.Start_message)
	_, err := bot.Send(answer)
	return err
}

func handleCommandStop(update tgbotapi.Update) error {
	answer := tgbotapi.NewMessage(update.Message.Chat.ID, texts.Stop_message)
	_, err := bot.Send(answer)
	if err == nil {
		database.DeactivateGroup(update.Message.Chat.ID)
	}
	return err
}

func sendPoll(update tgbotapi.Update, msgText string) error {
	answer := tgbotapi.NewMessage(update.Message.Chat.ID, msgText)
	replyMarkup := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(texts.Button_yes, "yes_participant"),
			tgbotapi.NewInlineKeyboardButtonData(texts.Button_no, "no_participant"),
		),
	)
	answer.ReplyMarkup = replyMarkup
	msg, err := bot.Send(answer)
	if err == nil {
		invalidatedPoll := database.AddOrUpdateGroup(update.Message.Chat.ID, msg.MessageID)
		if invalidatedPoll != nil {
			_, err := bot.Send(invalidatedPoll)
			if err != nil {
				log.Println(err)
			}
		}
	}
	return err
}

func handleCallbackQuery(update tgbotapi.Update) {
	if update.CallbackQuery.Data == "yes_participant" {
		handleNewParticipant(update)
	} else if update.CallbackQuery.Data == "no_participant" {
		handleDeleteParticipant(update)
	}
}

func handleNewParticipant(update tgbotapi.Update) {
	database.AddParticipant(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.From.ID)
	_, err := bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, texts.New_participant_message))
	if err != nil {
		log.Println(err)
	}
}

func handleDeleteParticipant(update tgbotapi.Update) {
	database.RemoveParticipant(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.From.ID)
	_, err := bot.AnswerCallbackQuery(tgbotapi.NewCallback(update.CallbackQuery.ID, texts.Delete_participant_message))
	if err != nil {
		log.Println(err)
	}
}

func main() {
	var err error
	bot, err = tgbotapi.NewBotAPI(cfg.Webhook.ApiToken)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	_, err = bot.SetWebhook(tgbotapi.NewWebhook(cfg.Webhook.Url + bot.Token))
	if err != nil {
		log.Fatal(err)
	}

	updates := bot.ListenForWebhook("/" + bot.Token)
	go func() {
		err := http.ListenAndServe("0.0.0.0:8443", nil)
		if err != nil {
			log.Fatal(err)
		}
	}()

	for update := range updates {
		log.Printf("%+v\n", update)
		if update.Message != nil {
			handleMessage(update)
		} else if update.CallbackQuery != nil {
			handleCallbackQuery(update)
		}
	}
}
