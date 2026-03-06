import MapKit
import SwiftUI

struct MapContainerView: View {
    let trip: Trip

    @State private var route: MKRoute?
    @State private var position: MapCameraPosition = .automatic

    var body: some View {
        Map(position: $position) {
            if trip.hasOrigin {
                Annotation(trip.originName.isEmpty ? "Origin" : trip.originName, coordinate: trip.origin) {
                    ZStack {
                        Circle().fill(.blue).frame(width: 14, height: 14)
                        Circle().stroke(.white, lineWidth: 2).frame(width: 14, height: 14)
                    }
                }
            }

            Annotation(trip.destinationName, coordinate: trip.destination) {
                Image(systemName: "mappin.circle.fill")
                    .font(.system(size: 32))
                    .foregroundStyle(.red)
                    .shadow(radius: 2)
            }

            if let route {
                MapPolyline(route.polyline)
                    .stroke(.blue.opacity(0.7), lineWidth: 5)
            }
        }
        .task { await loadRoute() }
    }

    private func loadRoute() async {
        guard trip.hasOrigin else { return }

        let req = MKDirections.Request()
        req.source = MKMapItem(placemark: MKPlacemark(coordinate: trip.origin))
        req.destination = MKMapItem(placemark: MKPlacemark(coordinate: trip.destination))
        req.transportType = .automobile

        guard let response = try? await MKDirections(request: req).calculate() else { return }
        route = response.routes.first

        // Fit camera to show both points with padding.
        let rect = response.routes.first?.polyline.boundingMapRect ?? MKMapRect(
            origin: MKMapPoint(trip.origin),
            size: MKMapSize(width: 1, height: 1)
        )
        position = .rect(rect.insetBy(dx: -rect.size.width * 0.3, dy: -rect.size.height * 0.3))
    }
}
