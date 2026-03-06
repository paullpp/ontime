import UserNotifications

/// Schedules a local notification as a safety net in case the server push is delayed.
/// Updated on every silent ETA push. Cancelled when the server push arrives.
enum LocalFallback {
    private static let center = UNUserNotificationCenter.current()

    static func schedule(for trip: Trip) async {
        await cancel(for: trip.id)
        guard trip.isActive, trip.latestEtaSeconds > 0 else { return }

        let leaveAt = trip.shouldLeaveAt
        guard leaveAt > Date() else { return }

        let content = UNMutableNotificationContent()
        content.title = "Time to leave!"
        content.body = "Drive to \(trip.destinationName): \(formatETA(trip.latestEtaSeconds)). Depart by \(formatTime(leaveAt))."
        content.sound = .default
        content.interruptionLevel = .timeSensitive
        content.userInfo = ["trip_id": trip.id.uuidString, "type": "leave_now"]

        let trigger = UNTimeIntervalNotificationTrigger(
            timeInterval: max(leaveAt.timeIntervalSinceNow, 1),
            repeats: false
        )
        let request = UNNotificationRequest(
            identifier: notifID(trip.id),
            content: content,
            trigger: trigger
        )
        try? await center.add(request)
    }

    static func cancel(for tripID: UUID) async {
        center.removePendingNotificationRequests(withIdentifiers: [notifID(tripID)])
    }

    private static func notifID(_ id: UUID) -> String { "ontime.fallback.\(id)" }

    private static func formatETA(_ seconds: Int) -> String {
        "\((seconds + 59) / 60) min"
    }

    private static func formatTime(_ date: Date) -> String {
        let f = DateFormatter()
        f.timeStyle = .short
        return f.string(from: date)
    }
}
