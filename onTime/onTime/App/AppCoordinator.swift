import SwiftUI

@Observable
final class AppCoordinator {
    var navigationPath = NavigationPath()
    var activeSheet: Sheet?
    var currentUser: User?

    var isAuthenticated: Bool { currentUser != nil }

    enum Sheet: Identifiable, Hashable {
        case createTrip
        case settings
        var id: Self { self }
    }

    enum Route: Hashable {
        case tripDetail(Trip)
    }

    // MARK: - Navigation

    func navigate(to route: Route) {
        navigationPath.append(route)
    }

    func present(_ sheet: Sheet) {
        activeSheet = sheet
    }

    func dismissSheet() {
        activeSheet = nil
    }

    func pop() {
        guard !navigationPath.isEmpty else { return }
        navigationPath.removeLast()
    }

    func popToRoot() {
        navigationPath = NavigationPath()
    }

    // MARK: - Auth

    func signIn(user: User, accessToken: String, refreshToken: String) async {
        await TokenStore.shared.save(accessToken: accessToken, refreshToken: refreshToken)
        await TokenStore.shared.saveUser(user)
        await MainActor.run { currentUser = user }
    }

    func signOut() async {
        try? await APIClient.shared.logout()
        await TokenStore.shared.clearAll()
        await MainActor.run {
            currentUser = nil
            navigationPath = NavigationPath()
            activeSheet = nil
        }
    }

    // MARK: - Deep links

    func handleTripDeepLink(_ tripID: UUID) {
        Task {
            guard let trip = try? await APIClient.shared.getTrip(id: tripID) else { return }
            await MainActor.run {
                popToRoot()
                navigate(to: .tripDetail(trip))
            }
        }
    }

    // MARK: - Session restore

    func restoreSession() async {
        guard let user = await TokenStore.shared.loadUser(),
              await TokenStore.shared.accessToken != nil else { return }
        // Verify the stored token is still valid by attempting a refresh.
        do {
            let response = try await APIClient.shared.refreshTokens()
            await TokenStore.shared.save(accessToken: response.accessToken, refreshToken: response.refreshToken)
            await MainActor.run { currentUser = user }
        } catch {
            await TokenStore.shared.clearAll()
        }
    }
}
