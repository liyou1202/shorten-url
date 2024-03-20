(function () {
// API
    const listGroup = document.querySelector('.list-group');
    const shortenBtn = document.getElementById("shorten-btn")
    const modal = new bootstrap.Modal(document.getElementById('loadingModal'));
    const input = document.getElementById("input")

    function addListItem(redirectURL, originURL) {
        var newLink = document.createElement('a');
        newLink.href = '#';
        newLink.classList.add('list-group-item', 'list-group-item-action', 'd-flex', 'gap-3', 'py-3');
        newLink.setAttribute('aria-current', 'true');

        newLink.innerHTML = `
        <div class="d-flex gap-2 w-100 justify-content-between">
            <div>
                <h6 class="mb-0">${redirectURL}</h6>
                <p class="mb-0 opacity-75">${originURL}</p>
            </div>
        </div>
        `;

        //插入第二項
        let secondItem = listGroup.children[1];
        listGroup.insertBefore(newLink, secondItem);
    }

    shortenBtn.onclick = function () {
        let originURL = input.value
        let testedURL = ((str) => {
            try {
                return new URL(str);
            } catch (err) {
                return null;
            }
        })(originURL);
        if (testedURL == null) {
            alert(`Wrong URL: ${originURL}`)
            return
        }

        const parsedUrl = {
            "scheme": testedURL.protocol.replace(':', ''),
            "domain": testedURL.hostname,
            "path": testedURL.pathname
        };
        modal.show();

        Promise.all([
            new Promise(resolve => setTimeout(resolve, 2000)), // 2 秒後解析
            fetch('https://redirect.liyou-chen.com/shorten', {
                method: 'POST',
                body: JSON.stringify(parsedUrl)
            }).then(response => {

                return response.json();
            }).then(data => {
                if (data.isSuccess === "true" && data.shortenStr !== "") {
                    let redirectURL = new URL("https://redirect.liyou-chen.com/" + data.shortenStr)
                    addListItem(redirectURL, originURL)
                    bindListGroup()
                } else {
                    alert("Failed to shorten URL:", data.message);
                }
            }).catch(error => {
                console.error('Something wrong:', error);
            })
        ]).then(() => {
            modal.hide(); // 隱藏 modal
        });
    }

// Toast
    const liveToast = document.getElementById("liveToast");
    const toastText = document.getElementById("toastText");
    const toast = new bootstrap.Toast(liveToast);
    const goBtn = document.getElementById("goBtn")

    goBtn.onclick = function () {
        window.open(this.getAttribute('data-url'));
        toast.hide()
    };
    bindListGroup()

    function bindListGroup() {
        document.querySelectorAll('.list-group a').forEach(function (link) {
            link.addEventListener('click', function (event) {
                const linkStr = link.querySelector('h6').textContent;
                const textarea = document.createElement('textarea');
                textarea.value = linkStr;
                document.body.appendChild(textarea);
                textarea.select();
                document.execCommand('copy');
                document.body.removeChild(textarea);

                toastText.textContent = linkStr + "  has been copied!"
                goBtn.setAttribute('data-url', linkStr);
                toast.show();
            });
        });
    }
})();
