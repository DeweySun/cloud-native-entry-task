const loginPanel = document.querySelector("#login-panel");
const profilePanel = document.querySelector("#profile-panel");
const loginForm = document.querySelector("#login-form");
const nicknameForm = document.querySelector("#nickname-form");
const pictureForm = document.querySelector("#picture-form");
const logoutButton = document.querySelector("#logout");
const message = document.querySelector("#message");
const avatar = document.querySelector("#avatar");
const nickname = document.querySelector("#nickname");
const username = document.querySelector("#username");

async function api(path, options = {}) {
  const response = await fetch(path, {
    credentials: "same-origin",
    ...options,
    headers: {
      ...(options.body instanceof FormData ? {} : { "Content-Type": "application/json" }),
      ...(options.headers || {})
    }
  });
  const data = await response.json().catch(() => ({}));
  if (!response.ok) {
    const err = data.error || { message: "Request failed." };
    throw new Error(err.message);
  }
  return data;
}

function showMessage(text, isError = true) {
  message.textContent = text || "";
  message.style.color = isError ? "#9a3412" : "#166534";
}

function renderProfile(user) {
  loginPanel.classList.add("hidden");
  profilePanel.classList.remove("hidden");
  nickname.textContent = user.nickname;
  username.textContent = user.username;
  nicknameForm.elements.nickname.value = user.nickname;
  avatar.src = user.profile_picture_url || "/placeholder.svg";
}

loginForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  showMessage("");
  const form = new FormData(loginForm);
  try {
    const result = await api("/api/login", {
      method: "POST",
      body: JSON.stringify({
        username: form.get("username"),
        password: form.get("password")
      })
    });
    renderProfile(result.user);
    showMessage("Login successful.", false);
  } catch (err) {
    showMessage(err.message);
  }
});

nicknameForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  showMessage("");
  try {
    const user = await api("/api/me/nickname", {
      method: "PUT",
      body: JSON.stringify({ nickname: nicknameForm.elements.nickname.value })
    });
    renderProfile(user);
    showMessage("Nickname updated.", false);
  } catch (err) {
    showMessage(err.message);
  }
});

pictureForm.addEventListener("submit", async (event) => {
  event.preventDefault();
  showMessage("");
  const form = new FormData(pictureForm);
  try {
    const user = await api("/api/me/profile-picture", {
      method: "POST",
      body: form
    });
    renderProfile(user);
    showMessage("Profile picture updated.", false);
  } catch (err) {
    showMessage(err.message);
  }
});

logoutButton.addEventListener("click", async () => {
  await api("/api/logout", { method: "POST", body: "{}" }).catch(() => {});
  profilePanel.classList.add("hidden");
  loginPanel.classList.remove("hidden");
  showMessage("Logged out.", false);
});
