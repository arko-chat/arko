import Foundation
import UIKit
import LocalAuthentication
import MobileModule

class NativeBridgeImpl: NSObject, MobileMobileNativeBridge {

    weak var viewController: UIViewController?

    init(viewController: UIViewController) {
        self.viewController = viewController
    }

    func getDeviceID() throws -> String {
        return UIDevice.current.identifierForVendor?.uuidString ?? UUID().uuidString
    }

    func showNotification(_ title: String, body: String) throws {
        let content = UNMutableNotificationContent()
        content.title = title
        content.body = body
        content.sound = .default

        let request = UNNotificationRequest(
            identifier: UUID().uuidString,
            content: content,
            trigger: nil
        )
        UNUserNotificationCenter.current().add(request)
    }

    func authenticateBiometric(_ reason: String) throws -> Bool {
        let context = LAContext()
        var error: NSError?
        guard context.canEvaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, error: &error) else {
            return false
        }

        var success = false
        let semaphore = DispatchSemaphore(value: 0)
        context.evaluatePolicy(.deviceOwnerAuthenticationWithBiometrics, localizedReason: reason) { result, _ in
            success = result
            semaphore.signal()
        }
        semaphore.wait()
        return success
    }

    func shareText(_ text: String) throws {
        DispatchQueue.main.async {
            let vc = UIActivityViewController(activityItems: [text], applicationActivities: nil)
            self.viewController?.present(vc, animated: true)
        }
    }

    func getSafeAreaInsetsTop() throws -> Int {
        var inset = 0
        DispatchQueue.main.sync {
            inset = Int(viewController?.view.safeAreaInsets.top ?? 0)
        }
        return inset
    }

    func getSafeAreaInsetsRight() throws -> Int { return 0 }
    func getSafeAreaInsetsBottom() throws -> Int {
        var inset = 0
        DispatchQueue.main.sync {
            inset = Int(viewController?.view.safeAreaInsets.bottom ?? 0)
        }
        return inset
    }
    func getSafeAreaInsetsLeft() throws -> Int { return 0 }

    func openURL(_ url: String) throws {
        guard let u = URL(string: url) else { return }
        DispatchQueue.main.async { UIApplication.shared.open(u) }
    }

    func onAppBackground() throws {}
    func onAppForeground() throws {}
}
