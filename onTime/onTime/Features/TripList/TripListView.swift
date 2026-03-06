import SwiftUI

struct TripListView: View {
    @Environment(AppCoordinator.self) private var coordinator
    @State private var viewModel = TripListViewModel()

    var body: some View {
        Group {
            if viewModel.isLoading && viewModel.trips.isEmpty {
                ProgressView("Loading trips…")
            } else if viewModel.trips.isEmpty {
                emptyState
            } else {
                tripList
            }
        }
        .navigationTitle("My Trips")
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Button { coordinator.present(.createTrip) } label: {
                    Image(systemName: "plus")
                }
            }
            ToolbarItem(placement: .topBarLeading) {
                Button { coordinator.present(.settings) } label: {
                    Image(systemName: "person.circle")
                }
            }
        }
        .alert("Error", isPresented: .constant(viewModel.errorMessage != nil), actions: {
            Button("OK") { viewModel.errorMessage = nil }
        }, message: {
            Text(viewModel.errorMessage ?? "")
        })
        .task { await viewModel.load() }
        .refreshable { await viewModel.load() }
    }

    private var tripList: some View {
        List {
            ForEach(viewModel.trips) { trip in
                Button {
                    coordinator.navigate(to: .tripDetail(trip))
                } label: {
                    TripRowView(trip: trip)
                }
                .buttonStyle(.plain)
                .swipeActions(edge: .trailing, allowsFullSwipe: true) {
                    Button(role: .destructive) {
                        Task { await viewModel.cancel(trip) }
                    } label: {
                        Label("Cancel", systemImage: "xmark.circle.fill")
                    }
                }
            }
        }
        .listStyle(.insetGrouped)
    }

    private var emptyState: some View {
        VStack(spacing: 20) {
            Image(systemName: "map")
                .font(.system(size: 60))
                .foregroundStyle(.secondary)
            Text("No active trips")
                .font(.title2)
                .fontWeight(.semibold)
            Text("Tap + to create a trip and get notified\nwhen to leave.")
                .multilineTextAlignment(.center)
                .foregroundStyle(.secondary)
            Button("Create Trip") {
                coordinator.present(.createTrip)
            }
            .buttonStyle(.borderedProminent)
        }
        .padding()
    }
}
