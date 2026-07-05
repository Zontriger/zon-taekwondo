//go:build !windows

package main

// En sistemas no-Windows no hay bandeja del sistema ni consola que ocultar: el
// servidor corre en primer plano y se apaga por cierre del navegador o Ctrl+C.

func hideConsoleIfOwned() {}

func runTray(cfg trayConfig) {}

func stopTray() {}
