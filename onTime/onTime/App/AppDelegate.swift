import UIKit

final class AppDelegate: NSObject, UIApplicationDelegate {

    func application(
        _ application: UIApplication,
        didRegisterForRemoteNotificationsWithDeviceToken deviceToken: Data
    ) {
        NotificationManager.shared.setDeviceToken(deviceToken)
        Task {
            guard let token = NotificationManager.shared.deviceToken else { return }
            try? await APIClient.shared.registerDevice(apnsToken: token)
        }
    }

    func application(
        _ application: UIApplication,
        didFailToRegisterForRemoteNotificationsWithError error: Error
    ) {
        print("[APNs] Failed to register: \(error)")
    }

    /// Called for silent pushes (content-available: 1).
    func application(
        _ application: UIApplication,
        didReceiveRemoteNotification userInfo: [AnyHashable: Any],
        fetchCompletionHandler completionHandler: @escaping (UIBackgroundFetchResult) -> Void
    ) {
        guard
            let idStr = userInfo["trip_id"] as? String,
            let tripID = UUID(uuidString: idStr),
            let type = userInfo["type"] as? String
        else {
            completionHandler(.noData)
            return
        }

        switch type {
        case "eta_update":
            Task {
                if let trip = try? await APIClient.shared.getTrip(id: tripID) {
                    await LocalFallback.schedule(for: trip)
                }
                completionHandler(.newData)
            }
        case "leave_now":
            Task {
                await LocalFallback.cancel(for: tripID)
                completionHandler(.newData)
            }
        default:
            completionHandler(.noData)
        }
    }
}
