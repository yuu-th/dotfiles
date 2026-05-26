import Cocoa
import Foundation

// Calls the configured wake command whenever macOS wakes from sleep.
// Run as a launchd KeepAlive agent so the process stays alive indefinitely.

func runWakeCommand() {
    let command = ProcessInfo.processInfo.environment["PROJWM_WAKE_COMMAND"] ?? "projwm reconcile"
    let task = Process()
    task.launchPath = "/bin/sh"
    task.arguments = ["-lc", command]
    do {
        try task.run()
        task.waitUntilExit()
    } catch {
        fputs("wake-watcher: wake command failed: \(error)\n", stderr)
    }
}

NSWorkspace.shared.notificationCenter.addObserver(
    forName: NSWorkspace.didWakeNotification,
    object: nil,
    queue: .main
) { _ in
    runWakeCommand()
}

// Graceful shutdown on SIGTERM / SIGINT
signal(SIGTERM) { _ in exit(0) }
signal(SIGINT)  { _ in exit(0) }

NSApplication.shared.setActivationPolicy(.prohibited)
NSApplication.shared.run()
