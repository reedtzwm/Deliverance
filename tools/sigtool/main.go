// sigtool — подпись релизов для самообновления.
//
//	sigtool genkey <файл-ключа>      создать пару ключей, вывести публичный
//	sigtool sign <файл-ключа> <файл> создать <файл>.sig
//	sigtool verify <public-hex> <файл>  проверить <файл> по <файл>.sig
package main

import (
	"fmt"
	"os"

	"zapretgui/internal/updsig"
)

func die(err error) {
	fmt.Fprintln(os.Stderr, "ошибка:", err)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "команды: genkey | sign | verify")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "genkey":
		if len(os.Args) != 3 {
			die(fmt.Errorf("использование: genkey <файл-ключа>"))
		}
		if _, err := os.Stat(os.Args[2]); err == nil {
			die(fmt.Errorf("файл %s уже существует, перезаписывать ключ нельзя", os.Args[2]))
		}
		pub, priv, err := updsig.GenerateKey()
		if err != nil {
			die(err)
		}
		if err := os.WriteFile(os.Args[2], []byte(priv), 0o600); err != nil {
			die(err)
		}
		fmt.Println("public:", pub)
	case "sign":
		if len(os.Args) != 4 {
			die(fmt.Errorf("использование: sign <файл-ключа> <файл>"))
		}
		priv, err := os.ReadFile(os.Args[2])
		if err != nil {
			die(err)
		}
		sig, err := updsig.SignFile(string(priv), os.Args[3])
		if err != nil {
			die(err)
		}
		if err := os.WriteFile(os.Args[3]+".sig", []byte(sig), 0o644); err != nil {
			die(err)
		}
		fmt.Println("подписано:", os.Args[3]+".sig")
	case "verify":
		if len(os.Args) != 4 {
			die(fmt.Errorf("использование: verify <public-hex> <файл>"))
		}
		sig, err := os.ReadFile(os.Args[3] + ".sig")
		if err != nil {
			die(err)
		}
		if err := updsig.VerifyFile(os.Args[2], string(sig), os.Args[3]); err != nil {
			die(err)
		}
		fmt.Println("подпись верна")
	default:
		die(fmt.Errorf("неизвестная команда %q", os.Args[1]))
	}
}
