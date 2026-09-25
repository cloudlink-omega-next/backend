(function() {
  const API_BASE = '/api/v1';
  let stripe = null;

  async function apiRequest(endpoint, options = {}) {
    const response = await fetch(`${API_BASE}${endpoint}`, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });

    const data = await response.json();
    if (!response.ok) {
      throw new Error(data.result || 'Request failed');
    }
    return data;
  }

  async function loadPointsBalance() {
    try {
      const data = await apiRequest('/points');
      const balanceEl = document.getElementById('points-balance');
      if (balanceEl) {
        balanceEl.textContent = data.data.balance;
      }

      const checkinBtn = document.getElementById('points-checkin-btn');
      if (checkinBtn) {
        checkinBtn.disabled = !data.data.can_check_in;
        if (!data.data.can_check_in) {
          checkinBtn.textContent = t('points_checkin_already') || 'Already checked in';
        } else {
          checkinBtn.textContent = t('points_checkin') || 'Daily Check-in';
        }
      }
    } catch (error) {
      console.error('Failed to load points:', error);
    }
  }

  async function handleCheckIn() {
    try {
      const data = await apiRequest('/points/checkin', {
        method: 'POST',
      });

      const balanceEl = document.getElementById('points-balance');
      if (balanceEl) {
        balanceEl.textContent = data.data.new_balance;
      }

      const checkinBtn = document.getElementById('points-checkin-btn');
      if (checkinBtn) {
        checkinBtn.disabled = true;
        checkinBtn.textContent = t('points_checkin_already') || 'Already checked in';
      }

      alert(t('points_checkin_success') || 'Check-in successful! You earned 10 points.');
    } catch (error) {
      alert(error.message || 'Failed to check in.');
    }
  }

  async function handlePurchase() {
    const amount = prompt(t('points_purchase_amount') || 'Enter points amount (100-10000):', '1000');
    if (!amount) return;

    const price = parseFloat(prompt(t('points_purchase_price') || 'Enter price:', '9.99'));
    if (isNaN(price) || price <= 0) return;

    const currency = prompt(t('points_purchase_currency') || 'Enter currency (e.g., USD):', 'USD');
    if (!currency) return;

    try {
      const data = await apiRequest('/points/purchase/stripe', {
        method: 'POST',
        body: JSON.stringify({
          amount: parseInt(amount),
          currency: currency.toLowerCase(),
        }),
      });

      const checkoutUrl = data.data.url;
      if (checkoutUrl) {
        window.location.href = checkoutUrl;
      } else {
        alert(t('points_purchase_error') || 'Failed to create checkout session.');
      }
    } catch (error) {
      alert(error.message || 'Failed to create purchase.');
    }
  }

  function handleStripeSuccess() {
    const urlParams = new URLSearchParams(window.location.search);
    const sessionId = urlParams.get('session_id');
    if (sessionId) {
      alert(t('points_purchase_success') || 'Purchase successful! Points have been added to your account.');
      loadPointsBalance();
    }
  }

  function handleStripeCancel() {
    alert(t('points_purchase_cancelled') || 'Purchase was cancelled.');
  }

  async function loadStripe() {
    if (!stripe) {
      try {
        await new Promise((resolve, reject) => {
          const script = document.createElement('script');
          script.src = 'https://js.stripe.com/v3/';
          script.onload = resolve;
          script.onerror = reject;
          document.head.appendChild(script);
        });
        const cfg = await fetch('/api/v1/stripe/config').then(r => r.json());
        stripe = window.Stripe(cfg.publishable_key);
      } catch (error) {
        console.error('Failed to load Stripe:', error);
      }
    }
    return stripe;
  }

  document.addEventListener('DOMContentLoaded', function() {
    const checkinBtn = document.getElementById('points-checkin-btn');
    const purchaseBtn = document.getElementById('points-purchase-btn');

    if (checkinBtn) {
      checkinBtn.addEventListener('click', handleCheckIn);
    }

    if (purchaseBtn) {
      purchaseBtn.addEventListener('click', handlePurchase);
    }

    if (window.location.pathname.includes('/points/success')) {
      handleStripeSuccess();
    } else if (window.location.pathname.includes('/points/cancel')) {
      handleStripeCancel();
    }

    loadPointsBalance();
  });
})();
