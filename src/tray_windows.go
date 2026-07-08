//go:build windows

package main

import (
	"log"
	"time"
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
	procIsIconic              = user32DLL.NewProc("IsIconic")
)

const (
	swHide    = 0
	swShow    = 5
	swRestore = 9 // muestra y restaura desde minimizado
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

// mostrarConsola muestra u oculta la ventana de consola propia. Al mostrar usa
// swRestore para des-minimizarla si venía enviada a la bandeja.
func mostrarConsola(show bool) {
	if consoleHWND == 0 {
		return
	}
	if show {
		procShowWindow.Call(consoleHWND, swRestore)
		procSetForegroundWindow.Call(consoleHWND)
		consoleVisible = true
	} else {
		procShowWindow.Call(consoleHWND, swHide)
		consoleVisible = false
	}
	actualizarTituloConsola()
}

// esIconica indica si la ventana está minimizada (IsIconic != 0).
func esIconica(hwnd uintptr) bool {
	r, _, _ := procIsIconic.Call(hwnd)
	return r != 0
}

// vigilarConsola detecta cuándo el usuario minimiza manualmente la consola y la
// envía a la bandeja (la oculta del taskbar) en lugar de dejarla minimizada. La
// consola se recupera desde el menú de la bandeja o haciendo clic en el icono.
func vigilarConsola() {
	if consoleHWND == 0 {
		return
	}
	go func() {
		for {
			time.Sleep(400 * time.Millisecond)
			if consoleHWND == 0 {
				return
			}
			if consoleVisible && esIconica(consoleHWND) {
				procShowWindow.Call(consoleHWND, swHide)
				consoleVisible = false
				actualizarTituloConsola()
			}
		}
	}()
}

// --- Icono de la bandeja del sistema -------------------------------------------

var trayCfg trayConfig

// mConsole es la opción de menú "Mostrar/Ocultar consola" (nil si no hay consola
// propia). Se guarda a nivel de paquete para poder actualizar su título tanto
// desde el menú como desde la vigilancia de minimizado.
var mConsole *systray.MenuItem

// actualizarTituloConsola sincroniza el texto del menú con el estado real.
func actualizarTituloConsola() {
	if mConsole == nil {
		return
	}
	if consoleVisible {
		mConsole.SetTitle("Ocultar consola")
	} else {
		mConsole.SetTitle("Mostrar consola")
	}
}

// runTray arranca el icono de la bandeja en su propia goroutine (systray.Run
// bloquea y gestiona el bucle de mensajes de Win32). Un clic sobre el icono
// muestra la consola; un clic sobre el icono también abre el menú (Abrir/Salir).
func runTray(cfg trayConfig) {
	trayCfg = cfg
	vigilarConsola() // enviar la consola a la bandeja cuando se minimice
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

	// Clic sobre el icono de la bandeja: mostrar la consola (si la controlamos).
	if tenemosConsola() {
		systray.SetOnTapped(func() { mostrarConsola(true) })
	}

	mOpen := systray.AddMenuItem("Abrir sistema", "Abrir el sistema en el navegador")

	// "Mostrar/Ocultar consola": solo si controlamos una consola propia (doble
	// clic). Si no, el canal queda nil y su caso del select nunca dispara.
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
			case <-mQuit.ClickedCh:
				if trayCfg.onQuit != nil {
					trayCfg.onQuit()
				}
				return
			}
		}
	}()
}
