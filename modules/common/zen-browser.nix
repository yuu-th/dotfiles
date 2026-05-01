# modules/common/zen-browser.nix
{ config, lib, pkgs, inputs, ... }:
let
  cfg = config.myConfig.zenBrowser;
  firefox-addons = inputs.firefox-addons.packages.${pkgs.stdenv.hostPlatform.system};
in {
  options.myConfig.zenBrowser.enable = lib.mkEnableOption "Zen Browser";

  config = lib.mkIf cfg.enable {
    home-manager.sharedModules = [ inputs.zen-browser.homeModules.default ];

    home-manager.users.${config.myConfig.primaryUser} = {
      programs.zen-browser = {
        enable = true;
        profiles.default = {
          id        = 0;
          isDefault = true;

          search = {
            force = true;
            engines = {
              "Perplexity" = {
                urls = [{ template = "https://www.perplexity.ai/search?q={searchTerms}"; }];
                keyword = "p";
              };
            };
          };

          extensions.packages = with firefox-addons; [
            ublock-origin
          ];

          # Rapidfox v7.3 — speed-first preset for 16GB+ Apple Silicon Mac.
          settings = {
            # ── Network: connections ──
            "network.http.max-connections" = 1200;
            "network.http.max-persistent-connections-per-server" = 8;
            "network.http.max-urgent-start-excessive-connections-per-host" = 5;
            "network.http.request.max-start-delay" = 5;
            "network.http.pacing.requests.burst" = 32;
            "network.http.pacing.requests.min-parallelism" = 10;
            "network.http.pacing.requests.enabled" = false;
            "network.dnsCacheExpiration" = 600;
            "network.dnsCacheExpirationGracePeriod" = 120;
            "network.dnsCacheEntries" = 10000;
            "network.ssl_tokens_cache_capacity" = 32768;

            # ── Network: speculative / prefetch (speed-first: enabled) ──
            "network.http.speculative-parallel-limit" = 10;
            "network.dns.disablePrefetch" = false;
            "network.dns.disablePrefetchFromHTTPS" = false;
            "network.prefetch-next" = true;
            "network.predictor.enabled" = true;
            "network.predictor.enable-prefetch" = true;
            "browser.urlbar.speculativeConnect.enabled" = true;
            "browser.places.speculativeConnect.enabled" = true;

            # ── Memory: JS GC ──
            "javascript.options.mem.high_water_mark" = 128;

            # ── Cache: RAM-only (16GB+) ──
            "browser.cache.disk.enable" = false;
            "browser.cache.disk.capacity" = 0;
            "browser.cache.memory.capacity" = 131072;
            "browser.cache.disk.smart_size.enabled" = false;
            "browser.cache.memory.max_entry_size" = 32768;
            "browser.cache.disk.metadata_memory_limit" = 16384;
            "browser.cache.max_shutdown_io_lag" = 100;

            # ── Image memory ──
            "image.mem.max_decoded_image_kb" = 512000;
            "image.cache.size" = 10485760;
            "image.mem.decode_bytes_at_a_time" = 65536;
            "image.mem.shared.unmap.min_expiration_ms" = 90000;

            # ── Media cache (16GB+) ──
            "media.memory_cache_max_size" = 1048576;
            "media.memory_caches_combined_limit_kb" = 4194304;
            "media.cache_readahead_limit" = 600;
            "media.cache_resume_threshold" = 300;

            # ── Session / storage ──
            "dom.storage.default_quota" = 20480;
            "dom.storage.shadow_writes" = true;
            "browser.sessionstore.interval" = 60000;
            "browser.sessionhistory.max_total_viewers" = 10;
            "browser.sessionstore.max_tabs_undo" = 10;
            "browser.sessionstore.max_entries" = 10;
            "browser.tabs.min_inactive_duration_before_unload" = 600000;

            # ── Content parsing / layout (16GB+ aggressive) ──
            "content.notify.interval" = 50000;
            "content.max.tokenizing.time" = 2000000;
            "content.switch.threshold" = 300000;
            "content.maxtextrun" = 8191;
            "content.interrupt.parsing" = true;
            "content.notify.ontimer" = true;
            "layout.frame_rate" = -1;
            "nglayout.initialpaint.delay" = 5;
            "gfx.content.skia-font-cache-size" = 32;

            # ── GPU / WebRender ──
            "gfx.webrender.all" = true;
            "gfx.webrender.enabled" = true;
            "gfx.webrender.compositor" = true;
            "gfx.webrender.precache-shaders" = true;
            "gfx.webrender.software" = false;
            "gfx.canvas.accelerated.cache-items" = 32768;
            "gfx.canvas.accelerated.cache-size" = 4096;
            "gfx.canvas.max-size" = 16384;
            "webgl.max-size" = 16384;
            "dom.webgpu.enabled" = true;
            "layers.acceleration.force-enabled" = true;
            "webgl.force-enabled" = true;

            # ── UI / scroll ──
            "ui.submenuDelay" = 0;
            "browser.uidensity" = 0;
            "dom.element.animate.enabled" = true;
            "general.smoothScroll" = true;
            "general.smoothScroll.msdPhysics.enabled" = false;
            "general.smoothScroll.currentVelocityWeighting" = 0;
            "apz.overscroll.enabled" = false;
            "general.smoothScroll.stopDecelerationWeighting" = 1;
            "general.smoothScroll.mouseWheel.durationMaxMS" = 150;
            "general.smoothScroll.mouseWheel.durationMinMS" = 50;
            "mousewheel.min_line_scroll_amount" = 18;
            "mousewheel.scroll_series_timeout" = 10;

            # ── Process / Fission (Advanced; mitigated by uBlock Origin) ──
            "fission.autostart" = false;
            "dom.ipc.processCount" = 8;
            "dom.ipc.keepProcessesAlive.web" = 4;
            "dom.ipc.processPriorityManager.backgroundUsesEcoQoS" = false;

            # ── Media / codecs ──
            "dom.media.webcodecs.h265.enabled" = true;
            "media.videocontrols.picture-in-picture.enable-when-switching-tabs.enabled" = true;

            # ── Accessibility / safebrowsing ──
            "accessibility.force_disabled" = 1;
            "browser.safebrowsing.downloads.remote.enabled" = false;

            # ── Privacy (speed-first trade-offs) ──
            "privacy.query_stripping.enabled" = true;
            "privacy.query_stripping.enabled.pbmode" = true;
            "privacy.partition.network_state" = false;
            "network.http.referer.XOriginPolicy" = 0;
            "network.http.referer.XOriginTrimmingPolicy" = 0;

            # ── macOS native ──
            "widget.macos.titlebar-blend" = true;
            "zen.haptic-feedback.enabled" = true;

            # ── AI / search suggestions ──
            "browser.ml.chat.enabled" = false;
            "browser.search.suggest.enabled" = false;
            "browser.urlbar.suggest.searches" = false;
            "browser.findBar.suggest.enabled" = false;
          };
        };
      };
    };
  };
}
