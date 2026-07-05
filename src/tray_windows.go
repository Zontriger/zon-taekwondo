//go:build windows

package main

import (
	"log"
	"unsafe"

	"fyne.io/systray"
	"golang.org/x/sys/windows"
)

// --- Consola: ocultar / mostrar ------------------------------------------------

var (
	kernel32DLL = windows.NewLazySystemDLL("kernel32.dll")
	user32DLL   = windows.NewLazySystemDLL("user32.dll")

	procGetConsoleWindow      = kernel32DLL.NewProc("GetConsoleWindow")
	procGetConsoleProcessList = kernel32DLL.NewProc("GetConsoleProcessList")
	procShowWindow            = user32DLL.NewProc("ShowWindow")
	procSetForegroundWindow   = user32DLL.NewProc("SetForegroundWindow")
)

const (
	swHide = 0
	swShow = 5
)

var (
	consoleHWND    uintptr // ventana de consola propia (0 si no la controlamos)
	consoleVisible bool
)

// hideConsoleIfOwned oculta la consola solo si la creó el propio proceso (al
// abrir el .exe con doble clic). Si el sistema se lanzó desde una consola
// existente (p. ej. cmd), esa consola es del usuario y NO se toca. Guarda el
// handle para poder alternar su visibilidad desde el menú de la bandeja.
func hideConsoleIfOwned() {
	hwnd, _, _ := procGetConsoleWindow.Call()
	if hwnd == 0 {
		return
	}
	var pids [4]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	if n <= 1 { // solo nosotros → consola propia del doble clic
		consoleHWND = hwnd
		procShowWindow.Call(hwnd, swHide)
		consoleVisible = false
	}
}

// tenemosConsola indica si controlamos una consola propia que se puede alternar.
func tenemosConsola() bool { return consoleHWND != 0 }

// mostrarConsola muestra u oculta la ventana de consola propia.
func mostrarConsola(show bool) {
	if consoleHWND == 0 {
		return
	}
	if show {
		procShowWindow.Call(consoleHWND, swShow)
		procSetForegroundWindow.Call(consoleHWND)
		consoleVisible = true
	} else {
		procShowWindow.Call(consoleHWND, swHide)
		consoleVisible = false
	}
}

// --- Icono de la bandeja del sistema -------------------------------------------

var trayCfg trayConfig

// runTray arranca el icono de la bandeja en su propia goroutine (systray.Run
// bloquea y gestiona el bucle de mensajes de Win32). Un clic sobre el icono
// abre el menú.
func runTray(cfg trayConfig) {
	trayCfg = cfg
	go func() {
		// La bandeja es opcional: si falla (p. ej. sin sesión de escritorio),
		// se registra y el servidor sigue funcionando igual.
		defer func() {
			if r := recover(); r != nil {
				log.Printf("bandeja del sistema no disponible: %v", r)
			}
		}()
		systray.Run(trayOnReady, func() {})
	}()
}

// stopTray retira el icono de la bandeja y termina su bucle.
func stopTray() { systray.Quit() }

func trayOnReady() {
	if len(trayCfg.iconICO) > 0 {
		systray.SetIcon(trayCfg.iconICO)
	}
	systray.SetTitle("SIGAT")
	systray.SetTooltip(trayCfg.tooltip)

	mOpen := systray.AddMenuItem("Abrir sistema", "Abrir el sistema en el navegador")

	// "Mostrar/Ocultar consola": solo si controlamos una consola propia (doble
	// clic). Si no, el canal queda nil y su caso del select nunca dispara.
	var mConsole *systray.MenuItem
	var consoleCh <-chan struct{}
	if tenemosConsola() {
		mConsole = systray.AddMenuItem("Mostrar consola", "Mostrar u ocultar la ventana de consola")
		consoleCh = mConsole.ClickedCh
	}

	mQuit := systray.AddMenuItem("Salir", "Cerrar el servidor y salir")

	go func() {
		for {
			select {
			case <-mOpen.ClickedCh:
				if trayCfg.onOpen != nil {
					trayCfg.onOpen()
				}
			case <-consoleCh:
				mostrarConsola(!consoleVisible)
				if consoleVisible {
					mConsole.SetTitle("Ocultar consola")
				} else {
					mConsole.SetTitle("Mostrar consola")
				}
			case <-mQuit.ClickedCh:
				if trayCfg.onQuit != nil {
					trayCfg.onQuit()
				}
				return
			}
		}
	}()
}
