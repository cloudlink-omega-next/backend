if (!window.__navbarInitialized) {
    window.__navbarInitialized = true;
    const mobileMenuButton = document.getElementById('mobile-menu-button');
    const mobileMenu = document.getElementById('mobile-menu');

    if (mobileMenuButton && mobileMenu) {
        mobileMenuButton.addEventListener('click', function () {
            const isExpanded = this.getAttribute('aria-expanded') === 'true';

            this.setAttribute('aria-expanded', !isExpanded);
            mobileMenu.classList.toggle('hidden');

            // Toggle icons
            const icons = this.getElementsByTagName('svg');
            if (icons[0]) icons[0].classList.toggle('hidden');
            if (icons[1]) icons[1].classList.toggle('hidden');
        });
    }

    const desktopDropdownButton = document.getElementById('desktop-dropdown-button');
    const desktopDropdownMenu = document.getElementById('desktop-dropdown-menu');

    if (desktopDropdownButton && desktopDropdownMenu) {
        desktopDropdownButton.addEventListener('click', function (e) {
            e.stopPropagation();
            desktopDropdownMenu.classList.toggle('hidden');
        });
    }

    const mobileDropdownButton = document.getElementById('mobile-dropdown-button');
    const mobileDropdownMenu = document.getElementById('mobile-dropdown-menu');

    if (mobileDropdownButton && mobileDropdownMenu) {
        mobileDropdownButton.addEventListener('click', function () {
            mobileDropdownMenu.classList.toggle('hidden');
        });
    }

    if (desktopDropdownButton && desktopDropdownMenu) {
        document.addEventListener('click', function (e) {
            if (!desktopDropdownButton.contains(e.target)) {
                desktopDropdownMenu.classList.add('hidden');
            }
        });
    }

    if (desktopDropdownMenu && mobileDropdownMenu) {
        document.addEventListener('keydown', function (e) {
            if (e.key === 'Escape') {
                desktopDropdownMenu.classList.add('hidden');
                mobileDropdownMenu.classList.add('hidden');
            }
        });
    }

    async function updateUnreadCount() {
        try {
            const res = await fetch('/api/v1/notifications/unread-count');
            if (res.ok) {
                const data = await res.json();
                if (data.result === 'OK' && data.data.count > 0) {
                    const navCount = document.getElementById('nav-unread-count');
                    const mobileCount = document.getElementById('mobile-unread-count');
                    if (navCount) {
                        navCount.textContent = data.data.count;
                        navCount.classList.remove('hidden');
                    }
                    if (mobileCount) {
                        mobileCount.textContent = data.data.count;
                        mobileCount.classList.remove('hidden');
                    }
                }
            }
        } catch (e) {
            console.error('Failed to fetch unread count:', e);
        }
    }

    if (document.getElementById('nav-unread-count') || document.getElementById('mobile-unread-count')) {
        updateUnreadCount();
        setInterval(updateUnreadCount, 60000);
    }

    const langToggle = document.getElementById('lang-toggle');
    const mobileLangToggle = document.getElementById('mobile-lang-toggle');

    function getI18nLangFallback() {
        return document.documentElement.lang === 'zh' ? 'zh' : 'en';
    }

    function toggleLanguage() {
        const getI18nLang = typeof window.getI18nLang === 'function' ? window.getI18nLang : getI18nLangFallback;
        const currentLang = getI18nLang();
        const newLang = currentLang === 'en' ? 'zh' : 'en';
        console.log('[lang-toggle] toggleLanguage clicked', { currentLang, newLang, hasSetI18nLang: typeof setI18nLang === 'function', hasApplyI18n: typeof applyI18n === 'function', htmlLang: document.documentElement.lang, savedLang: localStorage.getItem('language') });
        if (typeof setI18nLang === 'function') {
            setI18nLang(newLang);
        } else {
            document.documentElement.lang = newLang;
            localStorage.setItem('language', newLang);
            if (typeof applyI18n === 'function') {
                applyI18n();
            }
        }
        console.log('[lang-toggle] after switch', { htmlLang: document.documentElement.lang, savedLang: localStorage.getItem('language') });
    }

    if (langToggle) {
        langToggle.addEventListener('click', toggleLanguage);
    }

    if (mobileLangToggle) {
        mobileLangToggle.addEventListener('click', toggleLanguage);
    }
}