(function() {
    const scripts = [
        '/static/app_state.js',
        '/static/app_formatters.js',
        '/static/app_core.js',
        '/static/charts_usage.js',
        '/static/charts_model_work.js',
        '/static/charts_runtime.js',
        '/static/app_boot.js'
    ];

    function loadScript(src) {
        return new Promise((resolve, reject) => {
            const script = document.createElement('script');
            script.src = src;
            script.async = false;
            script.onload = resolve;
            script.onerror = () => reject(new Error(`Failed to load ${src}`));
            document.head.appendChild(script);
        });
    }

    async function loadDashboard() {
        if (!window.echarts) {
            throw new Error('图表资源加载失败，请确认 /static/echarts.min.js 可访问');
        }

        for (const script of scripts) {
            await loadScript(script);
        }
    }

    loadDashboard().catch(error => {
        const errorDiv = document.getElementById('errorMessage');
        if (errorDiv) {
            errorDiv.textContent = '加载页面脚本失败: ' + error.message;
            errorDiv.style.display = 'block';
        } else {
            console.error(error);
        }
    });
})();
