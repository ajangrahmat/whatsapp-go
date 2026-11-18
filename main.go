package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	_ "github.com/mattn/go-sqlite3"

	"github.com/mdp/qrterminal/v3"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"

	"google.golang.org/protobuf/proto"
)

// =================================
// KONFIGURASI BOT
// =================================

const (
	botName = "Go WhatsApp Bot"
)

// =================================
// FUNGSI BALASAN BOT
// =================================

func getReply(message string) string {

	message = strings.ToLower(strings.TrimSpace(message))

	switch message {

	case "halo", "hai", "hello":
		return "Halo juga! 👋\nSelamat datang di " + botName

	case "menu", "/menu":
		return `🤖 MENU WHATSAPP BOT

1. Profil
2. Bantuan
3. Kontak

Perintah:
- halo
- menu
- ping
- bantuan`

	case "ping":
		return "Pong! 🚀\nBot sedang aktif."

	case "1", "profil":
		return "👤 PROFIL BOT\n\nNama: Go WhatsApp Bot\nBahasa: Go\nLibrary: whatsmeow"

	case "2", "bantuan":
		return "🛟 BANTUAN\n\nSilakan ketik menu untuk melihat daftar perintah."

	case "3", "kontak":
		return "📞 KONTAK\n\nSilakan hubungi administrator."

	default:
		return "Maaf, perintah tidak dikenal. 😅\n\nKetik *menu* untuk melihat pilihan."
	}
}

// =================================
// EVENT HANDLER
// =================================

// Handler status koneksi
func connectionHandler() func(interface{}) {

	return func(evt interface{}) {

		switch v := evt.(type) {

		case *events.Connected:
			fmt.Println("Status      : Terhubung ke WhatsApp")

		case *events.Disconnected:
			fmt.Println("Status      : Koneksi terputus, mencoba menyambung ulang...")

		case *events.StreamReplaced:
			fmt.Println("Status      : Sesi digantikan proses lain (kredensial dipakai di tempat lain)")

		case *events.PairSuccess:
			fmt.Println("Status      : Perangkat berhasil ditautkan")
			fmt.Println("Perangkat   :", v.Platform)
			fmt.Println("Akun        :", v.ID.String())

		case *events.LoggedOut:
			fmt.Println("Status      : Sesi logout / tidak valid")
			fmt.Println("Penyebab    :", v.Reason)
			fmt.Println("Solusi      : hapus whatsapp.db lalu jalankan ulang untuk pairing.")
		}
	}
}

func messageHandler(client *whatsmeow.Client) func(interface{}) {

	return func(evt interface{}) {

		switch v := evt.(type) {

		case *events.Message:

			// Abaikan pesan dari akun bot sendiri
			if v.Info.IsFromMe {
				return
			}

			// Abaikan pesan kosong
			if v.Message == nil {
				return
			}

			// Ambil isi pesan teks
			message := v.Message.GetConversation()

			// Dukungan pesan teks extended
			if message == "" {
				message = v.Message.GetExtendedTextMessage().GetText()
			}

			if strings.TrimSpace(message) == "" {
				return
			}

			fmt.Println("--------------------------------")
			fmt.Println("Pesan masuk :", message)
			fmt.Println("Pengirim    :", v.Info.Sender.String())

			// Buat balasan
			reply := getReply(message)

			// Kirim balasan
			_, err := client.SendMessage(
				context.Background(),
				v.Info.Chat,
				&waProto.Message{
					Conversation: proto.String(reply),
				},
			)

			if err != nil {
				fmt.Println("Gagal mengirim pesan:", err)
				return
			}

			fmt.Println("Balasan     :", reply)
			fmt.Println("Status      : Terkirim")
			fmt.Println("--------------------------------")
		}
	}
}

// =================================
// MAIN PROGRAM
// =================================

func main() {

	ctx := context.Background()

	fmt.Println("================================")
	fmt.Println("     GO WHATSAPP BOT")
	fmt.Println("================================")

	// Identitas perangkat yang tampil di WhatsApp
	// (daftar "Perangkat tertaut" -> Google Chrome (Mac OS))
	store.SetOSInfo("Mac OS", [3]uint32{131, 0, 0})
	store.DeviceProps.PlatformType = waCompanionReg.DeviceProps_CHROME.Enum()

	// Database SQLite
	dbLog := waLog.Stdout("Database", "WARN", true)

	container, err := sqlstore.New(
		ctx,
		"sqlite3",
		"file:whatsapp.db?_foreign_keys=on",
		dbLog,
	)

	if err != nil {
		panic(err)
	}

	// Ambil perangkat tersimpan
	deviceStore, err := container.GetFirstDevice(ctx)

	if err != nil {
		panic(err)
	}

	// Buat WhatsApp client
	clientLog := waLog.Stdout("Client", "ERROR", true)

	client := whatsmeow.NewClient(
		deviceStore,
		clientLog,
	)

	// Daftarkan event handler
	client.AddEventHandler(connectionHandler())
	client.AddEventHandler(messageHandler(client))

	// =================================
	// LOGIN WHATSAPP
	// =================================

	if client.Store.ID == nil {

		fmt.Println("Login pertama kali...")
		fmt.Println("Menyiapkan QR Code...")

		qrChan, err := client.GetQRChannel(ctx)

		if err != nil {
			panic(err)
		}

		err = client.Connect()

		if err != nil {
			panic(err)
		}

		for evt := range qrChan {

			if evt.Event == "code" {

				fmt.Println()
				fmt.Println("SCAN QR CODE WHATSAPP")
				fmt.Println()

				qrterminal.GenerateHalfBlock(
					evt.Code,
					qrterminal.L,
					os.Stdout,
				)

			} else {

				fmt.Println("Login status:", evt.Event)
			}
		}

	} else {

		fmt.Println("Menggunakan sesi tersimpan...")

		err := client.Connect()

		if err != nil {
			panic(err)
		}
	}

	// =================================
	// BOT AKTIF
	// =================================

	fmt.Println()
	fmt.Println("================================")
	fmt.Println(" WHATSAPP BOT BERHASIL AKTIF")
	fmt.Println("================================")
	fmt.Println("Menunggu pesan masuk...")
	fmt.Println("Tekan CTRL+C untuk berhenti.")
	fmt.Println()

	// Tunggu sinyal penghentian
	stop := make(chan os.Signal, 1)

	signal.Notify(
		stop,
		os.Interrupt,
		syscall.SIGTERM,
	)

	<-stop

	fmt.Println()
	fmt.Println("Menghentikan WhatsApp Bot...")

	client.Disconnect()

	fmt.Println("Bot berhasil dihentikan.")
}
