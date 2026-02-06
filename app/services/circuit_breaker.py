# app/services/circuit_breaker.py
"""
Circuit Breaker Service

Implements the circuit breaker pattern to prevent cascading failures
when external services (Amadeus, Stripe, etc.) are unavailable.

States:
- CLOSED: Normal operation, requests pass through
- OPEN: Service is failing, requests fail fast
- HALF_OPEN: Testing if service has recovered

Configuration via environment variables:
- CIRCUIT_BREAKER_FAILURE_THRESHOLD: Number of failures before opening (default: 5)
- CIRCUIT_BREAKER_RECOVERY_TIMEOUT: Seconds before trying half-open (default: 30)
- CIRCUIT_BREAKER_SUCCESS_THRESHOLD: Successes needed to close (default: 3)
"""

from __future__ import annotations

import logging
import os
import time
from dataclasses import dataclass, field
from enum import Enum
from functools import wraps
from threading import Lock
from typing import Any, Callable, Dict, Optional, Type, TypeVar, Union

logger = logging.getLogger(__name__)

T = TypeVar("T")


class CircuitState(Enum):
    """Circuit breaker states."""
    CLOSED = "closed"
    OPEN = "open"
    HALF_OPEN = "half_open"


class CircuitBreakerError(Exception):
    """Raised when circuit breaker is open."""

    def __init__(self, service_name: str, message: Optional[str] = None):
        self.service_name = service_name
        self.message = message or f"Circuit breaker is open for {service_name}"
        super().__init__(self.message)


@dataclass
class CircuitBreakerConfig:
    """Configuration for a circuit breaker."""
    failure_threshold: int = 5  # Failures before opening
    recovery_timeout: float = 30.0  # Seconds before half-open
    success_threshold: int = 3  # Successes needed to close from half-open
    excluded_exceptions: tuple = ()  # Exceptions that don't trip the breaker


@dataclass
class CircuitBreakerState:
    """State tracking for a circuit breaker."""
    state: CircuitState = CircuitState.CLOSED
    failure_count: int = 0
    success_count: int = 0
    last_failure_time: float = 0.0
    last_error: Optional[Exception] = None
    lock: Lock = field(default_factory=Lock)


class CircuitBreaker:
    """
    Circuit breaker implementation for external service calls.

    Usage:
        breaker = CircuitBreaker("amadeus")

        @breaker
        def call_amadeus_api():
            ...

        # Or manually:
        with breaker:
            result = call_amadeus_api()
    """

    def __init__(
        self,
        name: str,
        failure_threshold: Optional[int] = None,
        recovery_timeout: Optional[float] = None,
        success_threshold: Optional[int] = None,
        excluded_exceptions: tuple = (),
    ):
        self.name = name

        # Load from env or use defaults
        self.config = CircuitBreakerConfig(
            failure_threshold=failure_threshold or int(
                os.getenv("CIRCUIT_BREAKER_FAILURE_THRESHOLD", "5")
            ),
            recovery_timeout=recovery_timeout or float(
                os.getenv("CIRCUIT_BREAKER_RECOVERY_TIMEOUT", "30")
            ),
            success_threshold=success_threshold or int(
                os.getenv("CIRCUIT_BREAKER_SUCCESS_THRESHOLD", "3")
            ),
            excluded_exceptions=excluded_exceptions,
        )

        self._state = CircuitBreakerState()

    @property
    def state(self) -> CircuitState:
        """Get current circuit state."""
        return self._state.state

    @property
    def is_closed(self) -> bool:
        """Check if circuit is closed (normal operation)."""
        return self._state.state == CircuitState.CLOSED

    @property
    def is_open(self) -> bool:
        """Check if circuit is open (failing fast)."""
        return self._state.state == CircuitState.OPEN

    def get_status(self) -> Dict[str, Any]:
        """Get circuit breaker status for monitoring."""
        return {
            "name": self.name,
            "state": self._state.state.value,
            "failure_count": self._state.failure_count,
            "success_count": self._state.success_count,
            "last_failure_time": self._state.last_failure_time,
            "last_error": str(self._state.last_error) if self._state.last_error else None,
            "config": {
                "failure_threshold": self.config.failure_threshold,
                "recovery_timeout": self.config.recovery_timeout,
                "success_threshold": self.config.success_threshold,
            },
        }

    def _should_allow_request(self) -> bool:
        """Check if request should be allowed through."""
        with self._state.lock:
            if self._state.state == CircuitState.CLOSED:
                return True

            if self._state.state == CircuitState.OPEN:
                # Check if recovery timeout has passed
                elapsed = time.time() - self._state.last_failure_time
                if elapsed >= self.config.recovery_timeout:
                    # Transition to half-open
                    self._state.state = CircuitState.HALF_OPEN
                    self._state.success_count = 0
                    logger.info(
                        f"Circuit breaker '{self.name}' transitioning to HALF_OPEN"
                    )
                    return True
                return False

            # HALF_OPEN: allow limited requests
            return True

    def _on_success(self) -> None:
        """Handle successful request."""
        with self._state.lock:
            if self._state.state == CircuitState.HALF_OPEN:
                self._state.success_count += 1
                if self._state.success_count >= self.config.success_threshold:
                    # Recovered, close the circuit
                    self._state.state = CircuitState.CLOSED
                    self._state.failure_count = 0
                    self._state.success_count = 0
                    logger.info(f"Circuit breaker '{self.name}' CLOSED (recovered)")
            elif self._state.state == CircuitState.CLOSED:
                # Reset failure count on success
                self._state.failure_count = 0

    def _on_failure(self, error: Exception) -> None:
        """Handle failed request."""
        # Check if this exception type should be excluded
        if isinstance(error, self.config.excluded_exceptions):
            return

        with self._state.lock:
            self._state.failure_count += 1
            self._state.last_failure_time = time.time()
            self._state.last_error = error

            if self._state.state == CircuitState.HALF_OPEN:
                # Failed during recovery, reopen
                self._state.state = CircuitState.OPEN
                logger.warning(
                    f"Circuit breaker '{self.name}' OPEN (failed during recovery): {error}"
                )
            elif self._state.state == CircuitState.CLOSED:
                if self._state.failure_count >= self.config.failure_threshold:
                    self._state.state = CircuitState.OPEN
                    logger.warning(
                        f"Circuit breaker '{self.name}' OPEN (threshold reached): {error}"
                    )

    def reset(self) -> None:
        """Manually reset the circuit breaker."""
        with self._state.lock:
            self._state.state = CircuitState.CLOSED
            self._state.failure_count = 0
            self._state.success_count = 0
            self._state.last_error = None
            logger.info(f"Circuit breaker '{self.name}' manually reset")

    def __call__(self, func: Callable[..., T]) -> Callable[..., T]:
        """Decorator to wrap a function with circuit breaker."""

        @wraps(func)
        def wrapper(*args: Any, **kwargs: Any) -> T:
            if not self._should_allow_request():
                raise CircuitBreakerError(
                    self.name,
                    f"Service '{self.name}' is temporarily unavailable. Please try again later.",
                )

            try:
                result = func(*args, **kwargs)
                self._on_success()
                return result
            except Exception as e:
                self._on_failure(e)
                raise

        return wrapper

    def __enter__(self) -> "CircuitBreaker":
        """Context manager entry."""
        if not self._should_allow_request():
            raise CircuitBreakerError(
                self.name,
                f"Service '{self.name}' is temporarily unavailable. Please try again later.",
            )
        return self

    def __exit__(
        self,
        exc_type: Optional[Type[BaseException]],
        exc_val: Optional[BaseException],
        exc_tb: Any,
    ) -> bool:
        """Context manager exit."""
        if exc_val is None:
            self._on_success()
        else:
            self._on_failure(exc_val)
        return False  # Don't suppress exceptions


# Global circuit breakers for external services
_circuit_breakers: Dict[str, CircuitBreaker] = {}


def get_circuit_breaker(
    name: str,
    failure_threshold: Optional[int] = None,
    recovery_timeout: Optional[float] = None,
    success_threshold: Optional[int] = None,
) -> CircuitBreaker:
    """
    Get or create a circuit breaker for a service.

    Args:
        name: Service name (e.g., "amadeus", "stripe", "google")
        failure_threshold: Override default failure threshold
        recovery_timeout: Override default recovery timeout
        success_threshold: Override default success threshold

    Returns:
        CircuitBreaker instance for the service
    """
    if name not in _circuit_breakers:
        _circuit_breakers[name] = CircuitBreaker(
            name=name,
            failure_threshold=failure_threshold,
            recovery_timeout=recovery_timeout,
            success_threshold=success_threshold,
        )
    return _circuit_breakers[name]


def get_all_circuit_breakers() -> Dict[str, Dict[str, Any]]:
    """Get status of all circuit breakers."""
    return {name: cb.get_status() for name, cb in _circuit_breakers.items()}


def reset_circuit_breaker(name: str) -> bool:
    """Reset a specific circuit breaker."""
    if name in _circuit_breakers:
        _circuit_breakers[name].reset()
        return True
    return False


def reset_all_circuit_breakers() -> None:
    """Reset all circuit breakers."""
    for cb in _circuit_breakers.values():
        cb.reset()


# Pre-configured circuit breakers for common services
amadeus_breaker = get_circuit_breaker(
    "amadeus",
    failure_threshold=5,
    recovery_timeout=60.0,  # 1 minute for travel API
    success_threshold=2,
)

stripe_breaker = get_circuit_breaker(
    "stripe",
    failure_threshold=3,  # More sensitive for payments
    recovery_timeout=30.0,
    success_threshold=2,
)

google_breaker = get_circuit_breaker(
    "google",
    failure_threshold=5,
    recovery_timeout=30.0,
    success_threshold=2,
)


# Convenience decorators
def with_amadeus_circuit_breaker(func: Callable[..., T]) -> Callable[..., T]:
    """Wrap function with Amadeus circuit breaker."""
    return amadeus_breaker(func)


def with_stripe_circuit_breaker(func: Callable[..., T]) -> Callable[..., T]:
    """Wrap function with Stripe circuit breaker."""
    return stripe_breaker(func)


def with_google_circuit_breaker(func: Callable[..., T]) -> Callable[..., T]:
    """Wrap function with Google circuit breaker."""
    return google_breaker(func)
