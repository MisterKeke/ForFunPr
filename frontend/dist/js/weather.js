import { els } from './dom.js';
import { escapeHtml } from './utils.js';
import {
  callGetStoredLocationWeather,
  callRefreshStoredLocationWeather,
  callGetWeather,
  callGetWeatherForCity,
} from './api.js';

let userLocationWeather = null;
let userLocationRequest = null;

function getWeatherDetails(code) {
  const weatherCode = Number(code);
  if (weatherCode === 0) return { icon: '&#9728;&#65039;', label: 'Clear sky' };
  if ([1, 2].includes(weatherCode)) return { icon: '&#127780;&#65039;', label: 'Partly cloudy' };
  if (weatherCode === 3) return { icon: '&#9729;&#65039;', label: 'Overcast' };
  if ([45, 48].includes(weatherCode)) return { icon: '&#127787;&#65039;', label: 'Foggy' };
  if ([51, 53, 55].includes(weatherCode)) return { icon: '&#127782;&#65039;', label: 'Drizzle' };
  if ([56, 57].includes(weatherCode)) return { icon: '&#127783;&#65039;', label: 'Freezing drizzle' };
  if ([61, 63, 65].includes(weatherCode)) return { icon: '&#127783;&#65039;', label: 'Rain' };
  if ([66, 67].includes(weatherCode)) return { icon: '&#127783;&#65039;', label: 'Freezing rain' };
  if ([71, 73, 75, 77].includes(weatherCode)) return { icon: '&#10052;&#65039;', label: 'Snow' };
  if ([80, 81, 82].includes(weatherCode)) return { icon: '&#127782;&#65039;', label: 'Rain showers' };
  if ([85, 86].includes(weatherCode)) return { icon: '&#127784;&#65039;', label: 'Snow showers' };
  if (weatherCode === 95) return { icon: '&#9928;&#65039;', label: 'Thunderstorm' };
  if ([96, 99].includes(weatherCode)) return { icon: '&#9928;&#65039;', label: 'Thunderstorm with hail' };
  return { icon: '&#127777;&#65039;', label: 'Weather unavailable' };
}

function formatTemperature(value) {
  const number = Number(value);
  return Number.isFinite(number) ? `${Math.round(number)}&deg;` : '&mdash;';
}

function formatWind(value) {
  const number = Number(value);
  return Number.isFinite(number) ? `${Math.round(number)} km/h` : '&mdash;';
}

function formatForecastDate(dateString, index) {
  if (index === 0) return 'Today';
  const date = new Date(`${dateString}T12:00:00`);
  if (Number.isNaN(date.getTime())) return dateString || '';
  return date.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
  });
}

function cityLabel(location) {
  if (!location) return 'Your location';
  const parts = [location.name, location.admin1, location.country].filter(
    (item, index, list) => item && list.indexOf(item) === index
  );
  return parts.join(', ') || 'Your location';
}

function requestBrowserLocation() {
  if (!navigator.geolocation) {
    return Promise.reject(new Error('Location services are not available in this app.'));
  }

  return new Promise((resolve, reject) => {
    navigator.geolocation.getCurrentPosition(resolve, reject, {
      enableHighAccuracy: false,
      maximumAge: 5 * 60 * 1000,
      timeout: 10000,
    });
  });
}

function getErrorMessage(error, fallback) {
  if (typeof error === 'string' && error.trim()) return error;
  if (error?.message) return error.message;
  return fallback;
}

function getLocationErrorMessage(error) {
  if (error?.code === 1) {
    return 'Location access was denied. Allow location access to see your local weather.';
  }
  if (error?.code === 2) {
    return 'Your location is unavailable right now. Please try again.';
  }
  if (error?.code === 3) {
    return 'Finding your location took too long. Please try again.';
  }
  return getErrorMessage(error, 'Your local weather could not be loaded.');
}

function formatWeatherUpdatedAt(updatedAt) {
  const date = new Date(updatedAt);
  if (Number.isNaN(date.getTime())) return '';

  return `Updated ${date.toLocaleString(undefined, {
    dateStyle: 'medium',
    timeStyle: 'short',
  })}`;
}

function userLocationWeatherResult(data, updatedAt) {
  return {
    data,
    locationName: data.timezone ? `Your location (${data.timezone})` : 'Your location',
    updatedAt,
  };
}

function renderForecast(container, data) {
  const current = data.current;
  const currentWeather = getWeatherDetails(current.weather_code);
  const days = data.daily.time.map((date, index) => {
    const weather = getWeatherDetails(data.daily.weather_code?.[index]);
    const precipitation = data.daily.precipitation_probability_max?.[index];
    const precipitationText = Number.isFinite(Number(precipitation)) ? `${Math.round(precipitation)}% rain` : '';

    return `
      <article class="weather-day${index === 0 ? ' weather-day-today' : ''}">
        <span class="weather-day-label">${escapeHtml(formatForecastDate(date, index))}</span>
        <span class="weather-day-icon" aria-hidden="true">${weather.icon}</span>
        <span class="weather-day-condition">${escapeHtml(weather.label)}</span>
        <span class="weather-day-temperatures"><strong>${formatTemperature(data.daily.temperature_2m_max?.[index])}</strong><span>${formatTemperature(data.daily.temperature_2m_min?.[index])}</span></span>
        ${precipitationText ? `<span class="weather-day-precipitation">${escapeHtml(precipitationText)}</span>` : ''}
      </article>
    `;
  }).join('');

  container.innerHTML = `
    <div class="weather-current">
      <div class="weather-current-icon" aria-hidden="true">${currentWeather.icon}</div>
      <div class="weather-current-main">
        <span class="weather-current-temperature">${formatTemperature(current.temperature_2m)}</span>
        <span class="weather-current-condition">${escapeHtml(currentWeather.label)}</span>
      </div>
      <div class="weather-current-details">
        <span>Feels like ${formatTemperature(current.apparent_temperature)}</span>
        <span>Wind ${formatWind(current.wind_speed_10m)}</span>
      </div>
    </div>
    <div class="weather-days" aria-label="Daily forecast">
      ${days}
    </div>
  `;
}

function renderSidebarWeather(weather) {
  const current = weather.data.current;
  const details = getWeatherDetails(current.weather_code);
  const today = weather.data.daily.time?.[0] || '';

  els.weatherSidebarDate.textContent = formatForecastDate(today, 0);
  els.weatherSidebarSummary.innerHTML = `
    <div class="weather-sidebar-location">${escapeHtml(weather.locationName)}</div>
    <div class="weather-sidebar-current">
      <span class="weather-sidebar-icon" aria-hidden="true">${details.icon}</span>
      <span class="weather-sidebar-temperature">${formatTemperature(current.temperature_2m)}</span>
      <span class="weather-sidebar-condition">${escapeHtml(details.label)}</span>
    </div>
    <div class="weather-sidebar-range">
      High ${formatTemperature(weather.data.daily.temperature_2m_max?.[0])} &middot; Low ${formatTemperature(weather.data.daily.temperature_2m_min?.[0])}
    </div>
  `;
  els.weatherSidebarLoading.classList.add('hidden');
  els.weatherSidebarError.classList.add('hidden');
  els.weatherSidebarSummary.classList.remove('hidden');
}

function renderUserLocationWeather(weather) {
  els.weatherLocationName.textContent = weather.locationName;
  els.weatherLocationUpdatedAt.textContent = formatWeatherUpdatedAt(weather.updatedAt);
  renderForecast(els.weatherLocationForecast, weather.data);
  els.weatherLocationLoading.classList.add('hidden');
  els.weatherLocationError.classList.add('hidden');
  els.weatherLocationForecast.classList.remove('hidden');
  renderSidebarWeather(weather);
}

function showUserLocationLoading() {
  els.weatherLocationName.textContent = 'Loading your local forecast...';
  els.weatherLocationUpdatedAt.textContent = '';
  els.weatherLocationLoading.classList.remove('hidden');
  els.weatherLocationError.classList.add('hidden');
  els.weatherLocationForecast.classList.add('hidden');
  els.weatherSidebarLoading.classList.remove('hidden');
  els.weatherSidebarError.classList.add('hidden');
  els.weatherSidebarSummary.classList.add('hidden');
  els.weatherSidebarDate.textContent = '';
}

function showUserLocationError(message) {
  els.weatherLocationName.textContent = 'Location unavailable';
  els.weatherLocationUpdatedAt.textContent = '';
  els.weatherLocationLoading.classList.add('hidden');
  els.weatherLocationForecast.classList.add('hidden');
  els.weatherLocationError.textContent = message;
  els.weatherLocationError.classList.remove('hidden');
  els.weatherSidebarLoading.classList.add('hidden');
  els.weatherSidebarSummary.classList.add('hidden');
  els.weatherSidebarError.textContent = message;
  els.weatherSidebarError.classList.remove('hidden');
  els.weatherSidebarDate.textContent = '';
}

function showCityMessage(message, type = 'error') {
  els.weatherCityResult.innerHTML = `<p class="weather-city-message weather-city-message-${type}">${escapeHtml(message)}</p>`;
  els.weatherCityResult.classList.remove('hidden');
}

async function searchCityWeather() {
  const query = els.weatherCitySearch.value.trim();
  if (!query) {
    showCityMessage('Enter a city name to search.', 'info');
    return;
  }

  const submitButton = els.weatherSearchForm.querySelector('button[type="submit"]');
  submitButton.disabled = true;
  els.weatherCityLoading.classList.remove('hidden');
  els.weatherCityResult.classList.add('hidden');

  try {
    const result = await callGetWeatherForCity(query);
    if (!result.found) {
      showCityMessage("Sorry, the city is not supported by api/doesn't exist", 'error');
      return;
    }

    const city = result.location;
    const forecast = result.weather;
    if (!forecast) {
      throw new Error('Weather data is not available for this city.');
    }
    const label = cityLabel(city);
    els.weatherCityResult.innerHTML = `
      <div class="weather-city-heading">
        <div>
          <h3>${escapeHtml(city.name)}</h3>
          <p>${escapeHtml(label)}</p>
        </div>
      </div>
      <div class="weather-forecast"></div>
    `;
    renderForecast(els.weatherCityResult.querySelector('.weather-forecast'), forecast);
    els.weatherCityResult.classList.remove('hidden');
  } catch (error) {
    showCityMessage(getErrorMessage(error, 'Weather for this city could not be loaded.'), 'error');
  } finally {
    els.weatherCityLoading.classList.add('hidden');
    submitButton.disabled = false;
  }
}

export function loadUserLocationWeather({ force = false } = {}) {
  if (userLocationRequest) return userLocationRequest;
  if (userLocationWeather && !force) {
    renderUserLocationWeather(userLocationWeather);
    return Promise.resolve(userLocationWeather);
  }

  if (!userLocationWeather) showUserLocationLoading();
  userLocationRequest = (async () => {
    try {
      const storedWeather = await callGetStoredLocationWeather();
      if (storedWeather.weather && !force) {
        // Render the last successful response before starting the network
        // request. Awaiting the refresh below does not block this UI update.
        userLocationWeather = userLocationWeatherResult(
          storedWeather.weather,
          storedWeather.updated_at
        );
        renderUserLocationWeather(userLocationWeather);

        try {
          const refreshedWeather = await callRefreshStoredLocationWeather();
          if (refreshedWeather.found && refreshedWeather.weather) {
            userLocationWeather = userLocationWeatherResult(
              refreshedWeather.weather,
              refreshedWeather.updated_at
            );
            renderUserLocationWeather(userLocationWeather);
          }
        } catch (error) {
          // A refresh failure must not replace a usable cached forecast with
          // an error. The next launch or manual refresh will try again.
          console.warn('Weather refresh failed; showing cached forecast:', error);
        }
        return userLocationWeather;
      }

      if (storedWeather.found) {
        const refreshedWeather = await callRefreshStoredLocationWeather();
        if (!refreshedWeather.weather) {
          throw new Error('Weather data is not available for your saved location.');
        }
        userLocationWeather = userLocationWeatherResult(
          refreshedWeather.weather,
          refreshedWeather.updated_at
        );
        renderUserLocationWeather(userLocationWeather);
        return userLocationWeather;
      }

      if (!storedWeather.found) {
        const position = await requestBrowserLocation();
        const { latitude, longitude } = position.coords;
        const data = await callGetWeather(latitude, longitude);
        userLocationWeather = userLocationWeatherResult(data, new Date().toISOString());
        renderUserLocationWeather(userLocationWeather);
        return userLocationWeather;
      }
    } catch (error) {
      if (userLocationWeather) {
        console.warn('Weather refresh failed; keeping the displayed forecast:', error);
        return userLocationWeather;
      }
      userLocationWeather = null;
      showUserLocationError(getLocationErrorMessage(error));
      return null;
    } finally {
      userLocationRequest = null;
    }
  })();

  return userLocationRequest;
}

export function initWeather() {
  els.weatherLocationRefresh.addEventListener('click', () => {
    loadUserLocationWeather({ force: true });
  });

  els.weatherSearchForm.addEventListener('submit', (event) => {
    event.preventDefault();
    searchCityWeather();
  });

  loadUserLocationWeather();
}
