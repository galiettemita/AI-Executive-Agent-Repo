from urllib.parse import urlparse
from app.schemas.assist import CheckoutPlanResponse

def build_checkout_plan(url: str, product_hint: str | None = None) -> CheckoutPlanResponse:
    domain = urlparse(url).netloc.lower()

    steps = [
        "Confirm you’re on the correct product page (size/color/model).",
        "Select any required options (size, color, quantity).",
        "Tap “Add to cart”.",
        "Open the cart and review items.",
        "Proceed to checkout.",
        "Log in or continue as guest (use iOS AutoFill / password manager).",
        "Confirm shipping address and delivery options.",
        "Choose payment method (Apple Pay if available).",
        "Review totals (tax/shipping) and tap “Place order” yourself."
    ]

    notes = []
    if "amazon." in domain:
        notes.append("Amazon often requires login; use your password manager/Keychain.")
    if "walmart." in domain:
        notes.append("Walmart pricing/availability can vary by ZIP/store; verify at checkout.")
    if product_hint:
        notes.append(f"Tip: double-check details for: {product_hint}")

    return CheckoutPlanResponse(retailer_domain=domain or "unknown", steps=steps, notes=notes)
