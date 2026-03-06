import AuthenticationServices
import Foundation

@Observable
final class AuthViewModel {
    var isLoading = false
    var errorMessage: String?

    private let coordinator: AppCoordinator
    private let notificationManager: NotificationManager

    init(coordinator: AppCoordinator, notificationManager: NotificationManager) {
        self.coordinator = coordinator
        self.notificationManager = notificationManager
    }

    // MARK: - Sign in with Apple

    func handleAuthorization(_ result: Result<ASAuthorization, Error>) async {
        switch result {
        case .success(let auth):
            guard let credential = auth.credential as? ASAuthorizationAppleIDCredential,
                  let tokenData = credential.identityToken,
                  let identityToken = String(data: tokenData, encoding: .utf8)
            else {
                errorMessage = "Failed to read Apple credentials."
                return
            }
            await signIn(identityToken: identityToken)

        case .failure(let error):
            // ASAuthorizationError.canceled (code 1001) means user dismissed; don't show error.
            let asError = error as? ASAuthorizationError
            if asError?.code != .canceled {
                errorMessage = error.localizedDescription
            }
        }
    }

#if DEBUG
    func debugSignIn() async {
        // Uses the server's MockAppleVerifier format: mock:<sub>:<email>
        await signIn(identityToken: "mock:debug_user_001:debug@ontime.dev")
    }
#endif

    // MARK: - Private

    private func signIn(identityToken: String) async {
        isLoading = true
        defer { isLoading = false }
        do {
            let deviceToken = notificationManager.deviceToken
            let response = try await APIClient.shared.signInWithApple(
                identityToken: identityToken,
                deviceToken: deviceToken
            )
            guard let user = response.user else {
                errorMessage = "Server did not return user info."
                return
            }
            await coordinator.signIn(
                user: user,
                accessToken: response.accessToken,
                refreshToken: response.refreshToken
            )
            // Request notification permission after successful sign-in.
            _ = await notificationManager.requestPermission()
        } catch {
            errorMessage = error.localizedDescription
        }
    }
}
