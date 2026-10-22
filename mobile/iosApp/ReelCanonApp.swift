import SwiftUI
import ReelCanon

@main
struct ReelCanonApp: App {
    var body: some Scene {
        WindowGroup { ReelCanonView().ignoresSafeArea() }
    }
}

struct ReelCanonView: UIViewControllerRepresentable {
    func makeUIViewController(context: Context) -> UIViewController {
        let apiBase = Bundle.main.object(forInfoDictionaryKey: "ReelCanonApiBaseUrl") as! String
        return MainViewControllerKt.MainViewController(apiBase: apiBase)
    }
    func updateUIViewController(_ controller: UIViewController, context: Context) {}
}
