# ToothTray-go

[English](README_en.md) | [Русский](README.md)

[![GitHub Repository](https://img.shields.io/badge/GitHub-internetson%2FToothTray--go-181717?logo=github)](https://github.com/internetson/ToothTray-go)
[![Release](https://img.shields.io/github/v/release/internetson/ToothTray-go?logo=github)](https://github.com/internetson/ToothTray-go/releases)
[![Arch](https://img.shields.io/badge/Arch-x64%20%7C%20ARM64-orange)](https://github.com/internetson/ToothTray-go/releases)
[![License](https://img.shields.io/badge/License-BSD_2--Clause-blue.svg)](LICENSE.md)
[![Go Version](https://img.shields.io/badge/Go-1.18+-00ADD8?logo=go)](https://go.dev/)
[![Platform](https://img.shields.io/badge/Platform-Windows_10%20%2F%2011-0078D6?logo=windows)](https://microsoft.com/windows)

Репозиторий проекта: **[https://github.com/internetson/ToothTray-go](https://github.com/internetson/ToothTray-go)**

**ToothTray-go** — современный аналог и улучшенная реализация утилиты ToothTray на языке **Go**.

Приложение работает исключительно в области уведомлений (System Tray) Windows и позволяет в один клик подключать и отключать сохранённые Bluetooth-устройства без лишних кликов, диалоговых окон и перехода в параметры системы.

---

## 📌 Авторство и связь с оригиналом

Этот проект является полной переработкой на языке Go оригинального проекта **[ToothTray](https://github.com/m2jean/ToothTray)**, созданного **[Jing Xia (m2jean)](https://github.com/m2jean)** и распространяемого под лицензией **BSD 2-Clause License**.

Оригинальная идея прямого обращения к драйверу Bluetooth через Windows Core Audio Topology API (`IKsControl`, `KSPROPSETID_BtAudio`) принадлежит автору оригинального репозитория. Данная реализация **ToothTray-go** развивает эту идею, устраняет технические ограничения оригинала и добавляет ряд удобных пользовательских возможностей.

---

## 🚀 Преимущества ToothTray-go перед оригинальным ToothTray (C++)

| Функция / Характеристика | Оригинал (m2jean/ToothTray, C++) | ToothTray-go |
| :--- | :--- | :--- |
| **Зависимости и среда** | Требует Visual Studio, C++/WinRT, пакеты `wil`, vcredist (`msvcp*.dll`) | **Чистый Go (Pure Go / Без CGO)**. Никаких внешних зависимостей, компилируется в один автономный `.exe` (~2.3 МБ). |
| **Блокировка интерфейса** | Вызовы `KsProperty` выполнялись синхронно в потоке окна — при задержке ответа драйвера трей зависал | **Полностью асинхронный**. Подключение и отключение выполняются в фоновых горутинах, меню закрывается мгновенно. |
| **Обратная связь (Уведомления)** | Отсутствует (результат можно было увидеть только в OutputDebugString) | **Системные уведомления в трее**. Сообщения о статусе: «Подключение к...», «Подключено», «Отключено», «Ошибка». |
| **Отображение состояния** | Отмечает подключение в меню, но привязано только к активному состоянию эндпоинта | **Двойная проверка статуса**. Синхронизирует данные из Win32 Bluetooth API и Core Audio; подключенные устройства наглядно отмечены галочкой `✓`. |
| **Поддержка устройств** | Только аудиоустройства с активным драйвером | **Все сопряжённые Bluetooth-устройства**. Аудиоустройства переключаются через быстрый Kernel Streaming; прочие устройства имеют fallback на управление службами Bluetooth. |
| **Поддержка архитектур** | Только x86/x64 сборки | **x64 (amd64) и native ARM64** (Snapdragon X Elite, Surface Pro ARM и др.). |
| **Автозагрузка** | Отсутствует (требовалось вручную создавать ярлык в `shell:startup`) | **Встроенный пункт «Запуск вместе с Windows»**. Включается и выключается в один клик через системный реестр (`HKCU\...\Run`). |
| **Окно консоли** | Зависит от конфигурации сборки Visual Studio | **Гарантированное отсутствие консоли**. Скомпилировано с флагом `-H windowsgui`, окно командной строки не появляется ни на мгновение. |
| **Защита от дубликатов** | Могло создавать дубликаты иконки при повторном клике на exe | **Системный мьютекс Windows**. При попытке повторного запуска программа тихо завершается, не плодя иконки в трее. |
| **Быстрый доступ к настройкам** | Отсутствует | **Пункт «Параметры Bluetooth...»**, открывающий `ms-settings:bluetooth` через Shell API без всплывающих окон CMD. |

---

## 💡 Как это работает

Windows предоставляет два стандартных способа подключения Bluetooth-аудио: панель *«Подключить»* (`Win + K`) и окно *«Параметры Bluetooth»*. Оба требуют нескольких кликов и ожидания.

Стандартные высокоуровневые API Windows не предоставляют прямого метода для быстрого программного переподключения. 

В **ToothTray-go** используется низкоуровневый механизм ядра:
1. Через **Windows Core Audio API** (`IMMDeviceEnumerator`) перечисляются аудио-эндпоинты рендеринга.
2. С помощью **Device Topology API** (`IDeviceTopology`, `IConnector`, `IPart`) находится фильтр Kernel Streaming (KS), подключённый к эндпоинту (проверяется префикс `{2}.\\?\bth` — `bthenum` / `bthhfenum`).
3. Через интерфейс `IKsControl` фильтру отправляется запрос свойства `KSPROPSETID_BtAudio` (`{7FA06C40-B8F6-4C7E-8556-E8C33A12E54D}`):
   * `KSPROPERTY_ONESHOT_RECONNECT` (0) — запрос драйверу на подключение;
   * `KSPROPERTY_ONESHOT_DISCONNECT` (1) — запрос драйверу на отключение.
4. Наушники с несколькими профилями (Stereo A2DP и Hands-Free HFP) группируются по общему `ContainerId` и переключаются одновременно.

---

## 🎮 Использование

1. Запустите `ToothTray-go.exe` (или `ToothTray-go-arm64.exe` на ARM64). В системном трее (рядом с часами) появится синяя иконка Bluetooth.
2. **Кликните по иконке** (правой или левой кнопкой мыши):
   * `✓ Название устройства` — устройство подключено (нажмите, чтобы отключить).
   * `  Название устройства` — устройство отключено (нажмите, чтобы подключить).
3. **Обновить список** — принудительно пересканирует устройства.
4. **Параметры Bluetooth...** — открывает стандартные настройки Bluetooth Windows.
5. **Запуск вместе с Windows** — включает или отключает автостарт приложения.
6. **Выход** — завершает работу программы и удаляет иконку из трея.

---

## 🔨 Сборка

### Требования:
* Установленный [Go](https://go.dev/) (версия 1.18 или новее).
* C-компилятор **не требуется** (Pure Go, сборка выполняется мгновенно).

### Клонирование и быстрая сборка (x64 и ARM64):
```cmd
git clone https://github.com/internetson/ToothTray-go.git
cd ToothTray-go
build.bat
```
Скрипт `build.bat` автоматически соберёт обе версии:
* `ToothTray-go.exe` — для Windows x64 (amd64)
* `ToothTray-go-arm64.exe` — для Windows ARM64 (Snapdragon X Elite, Surface Pro ARM и др.)

### Сборка вручную:

**Для x64 (amd64):**
```cmd
set GOOS=windows
set GOARCH=amd64
go build -ldflags="-H windowsgui -s -w" -o ToothTray-go.exe .
```

**Для ARM64:**
```cmd
set GOOS=windows
set GOARCH=arm64
go build -ldflags="-H windowsgui -s -w" -o ToothTray-go-arm64.exe .
```

*Для внедрения иконки в свойства исполняемых файлов:*
```cmd
go install github.com/akavel/rsrc@latest
rsrc -arch amd64 -ico ToothTray.ico -o rsrc_windows_amd64.syso
rsrc -arch arm64 -ico ToothTray.ico -o rsrc_windows_arm64.syso
```

---

## 📄 Лицензия и копирайт

Проект распространяется под условиями открытой лицензии **BSD 2-Clause License**:

* Copyright (c) 2023, **Jing Xia** (автор оригинального ToothTray: [github.com/m2jean/ToothTray](https://github.com/m2jean/ToothTray))
* Copyright (c) 2026, **ToothTray-go Contributors** ([github.com/internetson/ToothTray-go](https://github.com/internetson/ToothTray-go))

Полный текст лицензии приведён в файле [LICENSE.md](LICENSE.md).
