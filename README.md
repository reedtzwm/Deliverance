# Deliverance

Графическая оболочка для [zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube) под Windows 10/11:
сама скачивает и обновляет zapret, подбирает рабочую конфигурацию, живёт в трее.

## Возможности

- Автоматическая загрузка и обновление zapret с GitHub (без лимитов API).
- Автоподбор рабочего `.bat` по проверке Discord, YouTube и других адресов.
- Свои домены и готовые наборы для популярных игр, игровой фильтр.
- Встроенный локальный прокси для Telegram Desktop (порт на Go проекта [tg-ws-proxy](https://github.com/Flowseal/tg-ws-proxy), MIT).
- Диагностика сети с пояснением, что именно блокируется.
- Периодическая проверка с автоматическим переподбором, значок в трее, тёмная тема.
- Проверка подписи при самообновлении программы.

## Установка

Скачайте `Deliverance-Setup-<версия>.exe` из [Releases](../../releases) и запустите.
Программе нужны права администратора: драйвер WinDivert перехватывает трафик.

> Антивирусы иногда реагируют на `winws.exe` и WinDivert — это легитимные компоненты zapret.
> Если файлы пропали, добавьте `C:\ProgramData\Deliverance` в исключения.

## Сборка из исходников

Нужны: Go, Node.js, [Wails v2](https://wails.io), для установщика — [Inno Setup 6](https://jrsoftware.org/isinfo.php).

```powershell
go install github.com/wailsapp/wails/v2/cmd/wails@latest
.\build.ps1
```

Скрипт собирает приложение, упаковывает установщик в `build\installer` и подписывает его ключом обновлений
(путь задаётся переменной `UPDATE_KEY`, по умолчанию `%USERPROFILE%\.zapretgui-signing\update.key`).
Новый ключ создаётся командой `go run ./tools/sigtool genkey <файл>`; его публичную часть нужно вписать в
`updatePubKeyHex` в `selfupdate.go`. Приватный ключ никогда не добавляйте в репозиторий.

## Выпуск версии

1. Поднять версию в `selfupdate.go` (`appVersion`).
2. `.\build.ps1`
3. Создать релиз с тегом `v<версия>` и приложить оба файла:
   `Deliverance-Setup-<версия>.exe` и `Deliverance-Setup-<версия>.exe.sig`.

## Благодарности и лицензии

- [Flowseal/zapret-discord-youtube](https://github.com/Flowseal/zapret-discord-youtube) и [bol-van/zapret](https://github.com/bol-van/zapret) — обход блокировок. Скачиваются при первом запуске, в репозиторий и установщик не входят.
- [Flowseal/tg-ws-proxy](https://github.com/Flowseal/tg-ws-proxy) — идея и протокол Telegram-прокси (MIT, текст лицензии в `licenses/`).
- [Wails](https://wails.io), [fyne.io/systray](https://github.com/fyne-io/systray), [gorilla/websocket](https://github.com/gorilla/websocket).
