import UIKit
import WebKit
import Mobile

class ViewController: UIViewController, WKNavigationDelegate {

    private var webView: WKWebView!

    override func viewDidLoad() {
        super.viewDidLoad()

        webView = WKWebView(frame: view.bounds)
        webView.navigationDelegate = self
        webView.autoresizingMask = [.flexibleWidth, .flexibleHeight]
        view.addSubview(webView)

        let dataDir = FileManager.default
            .urls(for: .documentDirectory, in: .userDomainMask)[0]
            .path

        DispatchQueue.global(qos: .userInitiated).async {
            var error: NSError?
            guard let addr = MobileStart(dataDir, &error),
                  let url = URL(string: addr) else { return }
            DispatchQueue.main.async {
                self.webView.load(URLRequest(url: url))
            }
        }
    }

    func webView(
        _ webView: WKWebView,
        decidePolicyFor navigationAction: WKNavigationAction,
        decisionHandler: @escaping (WKNavigationActionPolicy) -> Void
    ) {
        let host = navigationAction.request.url?.host
        decisionHandler(host == "127.0.0.1" ? .allow : .cancel)
    }
}
