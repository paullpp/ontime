import SwiftUI

@main
struct onTimeApp: App {
    @UIApplicationDelegateAdaptor(AppDelegate.self) var appDelegate
    @State private var coordinator = AppCoordinator()
    @State private var notificationManager = NotificationManager.shared
    @State private var locationManager = LocationManager.shared

    var body: some Scene {
        WindowGroup {
            RootView()
                .environment(coordinator)
                .environment(notificationManager)
                .environment(locationManager)
                .task { await coordinator.restoreSession() }
                .onOpenURL { handleDeepLink($0) }
                .onChange(of: notificationManager.pendingTripDeepLink) { _, tripID in
                    if let tripID {
                        coordinator.handleTripDeepLink(tripID)
                        notificationManager.pendingTripDeepLink = nil
                    }
                }
        }
    }

    private func handleDeepLink(_ url: URL) {
        guard url.scheme == "ontime",
              url.host == "trip",
              let id = UUID(uuidString: url.lastPathComponent) else { return }
        coordinator.handleTripDeepLink(id)
    }
}

struct RootView: View {
    @Environment(AppCoordinator.self) private var coordinator

    var body: some View {
        if coordinator.isAuthenticated {
            AppView()
        } else {
            AuthView()
        }
    }
}

struct AppView: View {
    @Environment(AppCoordinator.self) private var coordinator

    var body: some View {
        @Bindable var coordinator = coordinator
        NavigationStack(path: $coordinator.navigationPath) {
            TripListView()
                .navigationDestination(for: AppCoordinator.Route.self) { route in
                    switch route {
                    case .tripDetail(let trip):
                        TripDetailView(trip: trip)
                    }
                }
        }
        .sheet(item: $coordinator.activeSheet) { sheet in
            switch sheet {
            case .createTrip:
                CreateTripView()
            case .settings:
                SettingsView()
            }
        }
    }
}
