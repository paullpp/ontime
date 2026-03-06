import AuthenticationServices
import SwiftUI

struct AuthView: View {
    @Environment(AppCoordinator.self) private var coordinator
    @Environment(NotificationManager.self) private var notificationManager
    @State private var viewModel: AuthViewModel?

    var body: some View {
        ZStack {
            // Background gradient
            LinearGradient(
                colors: [Color(.systemBlue).opacity(0.8), Color(.systemIndigo)],
                startPoint: .topLeading,
                endPoint: .bottomTrailing
            )
            .ignoresSafeArea()

            VStack(spacing: 48) {
                Spacer()

                // Logo / title
                VStack(spacing: 12) {
                    Image(systemName: "clock.arrow.circlepath")
                        .font(.system(size: 72))
                        .foregroundStyle(.white)
                    Text("onTime")
                        .font(.system(size: 40, weight: .bold))
                        .foregroundStyle(.white)
                    Text("Leave at the right moment.")
                        .font(.title3)
                        .foregroundStyle(.white.opacity(0.85))
                }

                Spacer()

                VStack(spacing: 16) {
                    if let vm = viewModel {
                        if vm.isLoading {
                            ProgressView()
                                .tint(.white)
                                .scaleEffect(1.3)
                                .frame(height: 50)
                        } else {
                            SignInWithAppleButton(.signIn) { request in
                                request.requestedScopes = [.fullName, .email]
                            } onCompletion: { result in
                                Task { await vm.handleAuthorization(result) }
                            }
                            .signInWithAppleButtonStyle(.white)
                            .frame(height: 50)
                            .cornerRadius(10)

#if DEBUG
                            Button("Debug Sign In") {
                                Task { await vm.debugSignIn() }
                            }
                            .foregroundStyle(.white.opacity(0.7))
                            .font(.footnote)
#endif
                        }

                        if let error = vm.errorMessage {
                            Text(error)
                                .font(.footnote)
                                .foregroundStyle(.white)
                                .multilineTextAlignment(.center)
                                .padding(.horizontal)
                        }
                    }
                }
                .padding(.horizontal, 32)
                .padding(.bottom, 48)
            }
        }
        .onAppear {
            viewModel = AuthViewModel(coordinator: coordinator, notificationManager: notificationManager)
        }
    }
}
