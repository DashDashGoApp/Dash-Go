// 05-popups-04-weather-icons.js — split from 05-popups-maps.js.
// Weather icons as inline SVG — font-independent so they render identically
// on the Pi regardless of which emoji font is (or isn't) installed.
// Each WMO code maps to [description, iconKey]; icon variants remain lightweight
// shared inline SVG strings, selected by the visual-style setting.
const WEATHER_ICON_SVG_SETS=Object.freeze({
  soft:Object.freeze({
    sun:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"12\" cy=\"12\" fill=\"#d9c074\" r=\"5\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></circle><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"18.7\" x2=\"20.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"16.7\" x2=\"18.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"12.0\" x2=\"12.0\" y1=\"18.7\" y2=\"20.5\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"7.3\" x2=\"6.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"5.3\" x2=\"3.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"7.3\" x2=\"6.0\" y1=\"7.3\" y2=\"6.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"12.0\" x2=\"12.0\" y1=\"5.3\" y2=\"3.5\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"16.7\" x2=\"18.0\" y1=\"7.3\" y2=\"6.0\"></line></svg>",
    partly:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"9\" cy=\"10\" fill=\"#d9c074\" r=\"4\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></circle><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"14.4\" x2=\"15.8\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"12.8\" x2=\"13.8\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"9.0\" x2=\"9.0\" y1=\"15.4\" y2=\"16.8\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"5.2\" x2=\"4.2\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"3.6\" x2=\"2.2\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"5.2\" x2=\"4.2\" y1=\"6.2\" y2=\"5.2\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"9.0\" x2=\"9.0\" y1=\"4.6\" y2=\"3.2\"></line><line stroke=\"#d9c074\" stroke-linecap=\"round\" stroke-width=\"1.35\" x1=\"12.8\" x2=\"13.8\" y1=\"6.2\" y2=\"5.2\"></line><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></svg>",
    cloud:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></svg>",
    overcast:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#9aa0ab\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><g transform=\"translate(-2 -3)\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></g></svg>",
    fog:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#9aa0ab\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"19\" y2=\"19\"></line><line stroke=\"#9aa0ab\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"21\" y2=\"21\"></line></svg>",
    drizzle:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"21.1\"></line></svg>",
    rain:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#9aa0ab\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#8bb4d4\" stroke-linecap=\"round\" stroke-width=\"1.6\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"22.1\"></line></svg>",
    snow:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#c4c8d0\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><g opacity=\".98\" stroke=\"#dfe6ee\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"8\" x2=\"8\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#dfe6ee\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"12\" x2=\"12\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#dfe6ee\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"16\" x2=\"16\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"22.2\" y2=\"20\"></line></g></svg>",
    storm:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-soft\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#9aa0ab\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><path d=\"M12 18l-2.5 3.5h2L11 24l3-4h-2z\" fill=\"#d9c074\" stroke=\"rgba(0,0,0,.18)\" stroke-width=\".25\"></path></svg>",
  }),
  bold:Object.freeze({
    sun:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"12\" cy=\"12\" fill=\"#ffd768\" r=\"5.8\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></circle><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"18.7\" x2=\"20.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"16.7\" x2=\"18.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"12.0\" x2=\"12.0\" y1=\"18.7\" y2=\"20.5\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"7.3\" x2=\"6.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"5.3\" x2=\"3.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"7.3\" x2=\"6.0\" y1=\"7.3\" y2=\"6.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"12.0\" x2=\"12.0\" y1=\"5.3\" y2=\"3.5\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"3\" x1=\"16.7\" x2=\"18.0\" y1=\"7.3\" y2=\"6.0\"></line></svg>",
    partly:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"9\" cy=\"10\" fill=\"#ffd768\" r=\"4\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></circle><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"14.4\" x2=\"15.8\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"12.8\" x2=\"13.8\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"9.0\" x2=\"9.0\" y1=\"15.4\" y2=\"16.8\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"5.2\" x2=\"4.2\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"3.6\" x2=\"2.2\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"5.2\" x2=\"4.2\" y1=\"6.2\" y2=\"5.2\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"9.0\" x2=\"9.0\" y1=\"4.6\" y2=\"3.2\"></line><line stroke=\"#ffd768\" stroke-linecap=\"round\" stroke-width=\"2.75\" x1=\"12.8\" x2=\"13.8\" y1=\"6.2\" y2=\"5.2\"></line><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></svg>",
    cloud:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></svg>",
    overcast:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9cce0\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><g transform=\"translate(-2 -3)\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></g></svg>",
    fog:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#b9cce0\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"19\" y2=\"19\"></line><line stroke=\"#b9cce0\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"21\" y2=\"21\"></line></svg>",
    drizzle:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"21.1\"></line></svg>",
    rain:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9cce0\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#72c8ff\" stroke-linecap=\"round\" stroke-width=\"2.7\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"22.1\"></line></svg>",
    snow:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#edf4ff\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"8\" x2=\"8\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"12\" x2=\"12\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"16\" x2=\"16\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"22.2\" y2=\"20\"></line></g></svg>",
    storm:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-bold\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9cce0\" stroke=\"rgba(26,38,54,.46)\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><path d=\"M12 18l-2.5 3.5h2L11 24l3-4h-2z\" fill=\"#ffdd5f\" stroke=\"rgba(0,0,0,.18)\" stroke-width=\".25\"></path></svg>",
  }),
  outline:Object.freeze({
    sun:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"12\" cy=\"12\" fill=\"rgba(0,0,0,.08)\" r=\"4.8\" stroke=\"#e5cf77\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></circle><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"18.7\" x2=\"20.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"16.7\" x2=\"18.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"12.0\" x2=\"12.0\" y1=\"18.7\" y2=\"20.5\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"7.3\" x2=\"6.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"5.3\" x2=\"3.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"7.3\" x2=\"6.0\" y1=\"7.3\" y2=\"6.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"12.0\" x2=\"12.0\" y1=\"5.3\" y2=\"3.5\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"16.7\" x2=\"18.0\" y1=\"7.3\" y2=\"6.0\"></line></svg>",
    partly:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"9\" cy=\"10\" fill=\"rgba(0,0,0,.08)\" r=\"4\" stroke=\"#e5cf77\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></circle><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"14.4\" x2=\"15.8\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"12.8\" x2=\"13.8\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"9.0\" x2=\"9.0\" y1=\"15.4\" y2=\"16.8\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"5.2\" x2=\"4.2\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"3.6\" x2=\"2.2\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"5.2\" x2=\"4.2\" y1=\"6.2\" y2=\"5.2\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"9.0\" x2=\"9.0\" y1=\"4.6\" y2=\"3.2\"></line><line stroke=\"#e5cf77\" stroke-linecap=\"round\" stroke-width=\"1.55\" x1=\"12.8\" x2=\"13.8\" y1=\"6.2\" y2=\"5.2\"></line><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path></svg>",
    cloud:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path></svg>",
    overcast:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#aeb8c7\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><g transform=\"translate(-2 -3)\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path></g></svg>",
    fog:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><line stroke=\"#aeb8c7\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"19\" y2=\"19\"></line><line stroke=\"#aeb8c7\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"21\" y2=\"21\"></line></svg>",
    drizzle:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"21.1\"></line></svg>",
    rain:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#aeb8c7\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#92c7ef\" stroke-linecap=\"round\" stroke-width=\"1.8\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"22.1\"></line></svg>",
    snow:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#cfd6e2\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><g opacity=\".98\" stroke=\"#edf5ff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"8\" x2=\"8\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#edf5ff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"12\" x2=\"12\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#edf5ff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"16\" x2=\"16\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"22.2\" y2=\"20\"></line></g></svg>",
    storm:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-outline\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"rgba(0,0,0,.08)\" stroke=\"#aeb8c7\" stroke-linejoin=\"round\" stroke-width=\"1.55\"></path><path d=\"M12 18l-2.5 3.5h2L11 24l3-4h-2z\" fill=\"#f4d35e\" stroke=\"rgba(0,0,0,.18)\" stroke-width=\".25\"></path></svg>",
  }),
  contrast:Object.freeze({
    sun:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"12\" cy=\"12\" fill=\"#ffe66b\" r=\"5.35\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></circle><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"18.7\" x2=\"20.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"16.7\" x2=\"18.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"12.0\" x2=\"12.0\" y1=\"18.7\" y2=\"20.5\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"7.3\" x2=\"6.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"5.3\" x2=\"3.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"7.3\" x2=\"6.0\" y1=\"7.3\" y2=\"6.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"12.0\" x2=\"12.0\" y1=\"5.3\" y2=\"3.5\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"16.7\" x2=\"18.0\" y1=\"7.3\" y2=\"6.0\"></line></svg>",
    partly:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"9\" cy=\"10\" fill=\"#ffe66b\" r=\"4\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></circle><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"14.4\" x2=\"15.8\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"12.8\" x2=\"13.8\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"9.0\" x2=\"9.0\" y1=\"15.4\" y2=\"16.8\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"5.2\" x2=\"4.2\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"3.6\" x2=\"2.2\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"5.2\" x2=\"4.2\" y1=\"6.2\" y2=\"5.2\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"9.0\" x2=\"9.0\" y1=\"4.6\" y2=\"3.2\"></line><line stroke=\"#ffe66b\" stroke-linecap=\"round\" stroke-width=\"2.35\" x1=\"12.8\" x2=\"13.8\" y1=\"6.2\" y2=\"5.2\"></line><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></svg>",
    cloud:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></svg>",
    overcast:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#dcecff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><g transform=\"translate(-2 -3)\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path></g></svg>",
    fog:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#dcecff\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"19\" y2=\"19\"></line><line stroke=\"#dcecff\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"21\" y2=\"21\"></line></svg>",
    drizzle:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"21.1\"></line></svg>",
    rain:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#dcecff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#24bfff\" stroke-linecap=\"round\" stroke-width=\"2.6\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"22.1\"></line></svg>",
    snow:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#f8fbff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"8\" x2=\"8\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"12\" x2=\"12\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#ffffff\" stroke-linecap=\"round\" stroke-width=\"1.45\"><line x1=\"16\" x2=\"16\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"22.2\" y2=\"20\"></line></g></svg>",
    storm:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-contrast\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#dcecff\" stroke=\"#182535\" stroke-linejoin=\"round\" stroke-width=\"0.78\"></path><path d=\"M12 18l-2.5 3.5h2L11 24l3-4h-2z\" fill=\"#fff14a\" stroke=\"rgba(0,0,0,.18)\" stroke-width=\".25\"></path></svg>",
  }),
  playful:Object.freeze({
    sun:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"12\" cy=\"12\" fill=\"#ffce5c\" r=\"5\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></circle><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"18.7\" x2=\"20.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"16.7\" x2=\"18.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"12.0\" x2=\"12.0\" y1=\"18.7\" y2=\"20.5\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"7.3\" x2=\"6.0\" y1=\"16.7\" y2=\"18.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"5.3\" x2=\"3.5\" y1=\"12.0\" y2=\"12.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"7.3\" x2=\"6.0\" y1=\"7.3\" y2=\"6.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"12.0\" x2=\"12.0\" y1=\"5.3\" y2=\"3.5\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"16.7\" x2=\"18.0\" y1=\"7.3\" y2=\"6.0\"></line></svg>",
    partly:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><circle cx=\"9\" cy=\"10\" fill=\"#ffce5c\" r=\"4\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></circle><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"14.4\" x2=\"15.8\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"12.8\" x2=\"13.8\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"9.0\" x2=\"9.0\" y1=\"15.4\" y2=\"16.8\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"5.2\" x2=\"4.2\" y1=\"13.8\" y2=\"14.8\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"3.6\" x2=\"2.2\" y1=\"10.0\" y2=\"10.0\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"5.2\" x2=\"4.2\" y1=\"6.2\" y2=\"5.2\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"9.0\" x2=\"9.0\" y1=\"4.6\" y2=\"3.2\"></line><line stroke=\"#ffce5c\" stroke-linecap=\"round\" stroke-width=\"1.65\" x1=\"12.8\" x2=\"13.8\" y1=\"6.2\" y2=\"5.2\"></line><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></svg>",
    cloud:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></svg>",
    overcast:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#8faed2\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><g transform=\"translate(-2 -3)\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path></g></svg>",
    fog:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#8faed2\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"19\" y2=\"19\"></line><line stroke=\"#8faed2\" stroke-linecap=\"round\" stroke-width=\"1.5\" x1=\"5\" x2=\"19\" y1=\"21\" y2=\"21\"></line></svg>",
    drizzle:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"21.1\"></line><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"21.1\"></line></svg>",
    rain:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#8faed2\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"8\" x2=\"6.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"12\" x2=\"10.4\" y1=\"18.4\" y2=\"22.1\"></line><line stroke=\"#68d8ff\" stroke-linecap=\"round\" stroke-width=\"1.9\" x1=\"16\" x2=\"14.4\" y1=\"18.4\" y2=\"22.1\"></line></svg>",
    snow:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#b9d7f6\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><g opacity=\".98\" stroke=\"#f4fbff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"8\" x2=\"8\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"6.1\" x2=\"9.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#f4fbff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"12\" x2=\"12\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"10.1\" x2=\"13.9\" y1=\"22.2\" y2=\"20\"></line></g><g opacity=\".98\" stroke=\"#f4fbff\" stroke-linecap=\"round\" stroke-width=\"1.1\"><line x1=\"16\" x2=\"16\" y1=\"18.9\" y2=\"23.3\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"20\" y2=\"22.2\"></line><line x1=\"14.1\" x2=\"17.9\" y1=\"22.2\" y2=\"20\"></line></g></svg>",
    storm:"<svg aria-hidden=\"true\" class=\"wxsvg wxsvg-playful\" focusable=\"false\" height=\"1em\" style=\"display:block;overflow:visible\" viewBox=\"0 0 24 24\" width=\"1em\"><path d=\"M7 17.5h9a3.5 3.5 0 0 0 .3-6.98A5 5 0 0 0 6.5 11 3.25 3.25 0 0 0 7 17.5z\" fill=\"#8faed2\" stroke=\"rgba(28,36,44,.26)\" stroke-linejoin=\"round\" stroke-width=\"0.35\"></path><path d=\"M12 18l-2.5 3.5h2L11 24l3-4h-2z\" fill=\"#ffe15c\" stroke=\"rgba(0,0,0,.18)\" stroke-width=\".25\"></path></svg>",
  }),
});
function buildWeatherIconSet(style){
  return WEATHER_ICON_SVG_SETS[style]||WEATHER_ICON_SVG_SETS.soft;
}
const ICON_CACHE={};
function iconSetForStyle(style){
  const key=(typeof WEATHER_ICON_STYLES!=="undefined" && WEATHER_ICON_STYLES[style]) ? style : "soft";
  if(!ICON_CACHE[key]) ICON_CACHE[key]=buildWeatherIconSet(key);
  return ICON_CACHE[key];
}
function currentWeatherIconSet(){
  const settings=typeof dashboardRuntimeSettings==="function"?dashboardRuntimeSettings():null;
  const s=(settings&&settings.weatherIconStyle)||CONFIG.weatherIconStyle||"soft";
  return iconSetForStyle(s);
}
function weatherIconFor(key){
  const set=currentWeatherIconSet();
  const fallback=iconSetForStyle("soft");
  return set[key]||set.cloud||fallback.cloud;
}
const WMO={0:["Clear","sun"],1:["Mostly clear","partly"],2:["Partly cloudy","partly"],3:["Overcast","overcast"],
45:["Fog","fog"],48:["Rime fog","fog"],51:["Light drizzle","drizzle"],53:["Drizzle","drizzle"],55:["Heavy drizzle","rain"],
61:["Light rain","drizzle"],63:["Rain","rain"],65:["Heavy rain","rain"],66:["Freezing rain","rain"],67:["Freezing rain","rain"],
71:["Light snow","snow"],73:["Snow","snow"],75:["Heavy snow","snow"],77:["Snow grains","snow"],
80:["Showers","drizzle"],81:["Showers","rain"],82:["Violent showers","storm"],85:["Snow showers","snow"],86:["Snow showers","snow"],
95:["Thunderstorm","storm"],96:["Thunderstorm w/ hail","storm"],99:["Severe thunderstorm","storm"]};
function wmo(c){ const e=WMO[c]||["—","cloud"]; return [e[0], weatherIconFor(e[1])]; }
function wmoSidebar(c){
  const brief={96:"Storm + hail",99:"Severe storms"};
  const [desc,ic]=wmo(c);
  return [brief[c]||desc,ic];
}

function weatherSourceUvText(dsrc,idx){
  if(idx<0 || !dsrc || !dsrc.uv_index_max) return "UV —";
  const raw=dsrc.uv_index_max[idx], uv=(typeof cleanUv==="function"?cleanUv(raw):raw);
  return uv==null ? "UV ignored" : "UV "+wxNum(uv,1);
}
function weatherSourceHealthText(status){
  const st=status||{};
  if(st.cacheHit){
    const age=Number(st.cacheAgeSeconds||st.providerCacheAgeSeconds||0);
    const label=st.stale?"stale cache":"cache";
    return age>0?label+" "+Math.max(1,Math.round(age/60))+"m":label;
  }
  if(Number(st.durationMs)>=0&&st.liveAttempted){
    const parts=[];
    const ms=Number(st.durationMs);
    parts.push(ms>=1000?(ms/1000).toFixed(1)+"s":Math.round(ms)+"ms");
    const calls=Number(st.networkCalls);
    if(Number.isFinite(calls)&&calls>0) parts.push(calls+" call"+(calls===1?"":"s"));
    const bytes=Number(st.responseBytes);
    if(Number.isFinite(bytes)&&bytes>0) parts.push((bytes/1024).toFixed(bytes>=10240?0:1)+" KB");
    const http=Number(st.httpStatus);
    if(Number.isFinite(http)&&http>0) parts.push("HTTP "+Math.round(http));
    return parts.join(" · ");
  }
  return "";
}
function refreshWeatherAfterSourceToggle(dateStr,fallbackIndex){
  if(!WX || !Array.isArray(WX._sources) || typeof blendWeatherSources!=="function") return;
  setWeatherPayload(typeof normalizeWeatherDayRollover==="function"?normalizeWeatherDayRollover(blendWeatherSources(WX._sources),new Date()):blendWeatherSources(WX._sources));
  if(typeof renderWeather==="function") renderWeather();
  const idx=weatherDailyIndexFor(dateStr);
  showWxDayPopup(idx>=0?idx:fallbackIndex);
}
function renderWeatherSourceLine(line,label,state,meta,detail){
  const top=el("div","wxsourcecardtop");
  top.append(el("b",null,label),el("strong",null,state));
  const bottom=el("div","wxsourcecardmeta");
  bottom.append(el("span",null,meta),el("span",null,detail));
  line.append(top,bottom);
}
function appendWeatherSourceDetails(body,i){
  if(typeof weatherSourceRowsForDay!=="function") return;
  const dateStr=WX&&WX.daily&&WX.daily.time&&WX.daily.time[i];
  const srcRows=weatherSourceRowsForDay(i);
  if(!srcRows.length) return;
  const card=el("details","wxsourcecompare wxsources wxsources-collapsed");
  card.open=false;
  const sourceCountText=srcRows.length+" source"+(srcRows.length===1?"":"s");
  const summary=el("summary","wxsourcehead");
  const updateSummary=()=>{
    const open=!!card.open;
    summary.textContent="Weather sources · "+sourceCountText+" · tap to "+(open?"hide":"show");
    summary.setAttribute("aria-label",(open?"Hide":"Show")+" weather source details. Source rows support double-tap to toggle inclusion.");
  };
  updateSummary();
  card.addEventListener("toggle",updateSummary);
  card.appendChild(summary);
  const grid=el("div","wxsourcesgrid");
  for(const src of srcRows){
    const idx=src.idx, dsrc=src.daily||{};
    const line=el("div","wxsourcerow"+(src.ok===false?" wxsourcefail":"")+(src.disabled?" wxsourcedisabled":""));
    line.dataset.sourceId=src.id||"";
    // Avoid native browser title tooltips on the kiosk WebKit path; they can leave
    // a ghosted tooltip/outline over modal content. The visible header carries
    // the instruction, while aria-label keeps the row descriptive.
    line.setAttribute("aria-label",src.disabled?"Weather source excluded. Double-tap to include this source":"Weather source included. Double-tap to exclude this source");
    if(src.ok===false){
      const state=src.disabled?"Excluded":"Unavailable";
      const detail=src.disabled?"double-tap to include":(src.error||"No response");
      const health=weatherSourceHealthText(src.status);
      renderWeatherSourceLine(line,src.label,state,[src.tier,health].filter(Boolean).join(" · "),detail);
    }else if(src.disabled){
      const health=weatherSourceHealthText(src.status);
      renderWeatherSourceLine(line,src.label,"Excluded",[src.tier,health].filter(Boolean).join(" · "),"double-tap to include");
    }else{
      const hi=idx>=0&&dsrc.temperature_2m_max?wxNum(dsrc.temperature_2m_max[idx],1)+"°":"—";
      const lo=idx>=0&&dsrc.temperature_2m_min?wxNum(dsrc.temperature_2m_min[idx],1)+"°":"—";
      const pp=idx>=0&&dsrc.precipitation_probability_max&&dsrc.precipitation_probability_max[idx]!=null?wxPercent(dsrc.precipitation_probability_max[idx]):"—";
      const total=idx>=0&&dsrc.precipitation_sum&&dsrc.precipitation_sum[idx]!=null?wxPrecipTotalText(dsrc.precipitation_sum[idx]).replace(" total",""):"—";
      const wind=idx>=0&&dsrc.wind_speed_10m_max&&dsrc.wind_speed_10m_max[idx]!=null?wxNum(dsrc.wind_speed_10m_max[idx],1)+" "+CONFIG.windUnit:"—";
      const uv=weatherSourceUvText(dsrc,idx);
      const health=weatherSourceHealthText(src.status);
      renderWeatherSourceLine(line,src.label,hi+" / "+lo,[src.tier,health].filter(Boolean).join(" · "),"rain "+pp+" · total "+total+" · wind "+wind+" · "+uv);
    }
    const toggle=()=>{
      if(typeof toggleWeatherSourceDisabled!=="function") return;
      if(toggleWeatherSourceDisabled(src.id,WX&&WX._sources)) refreshWeatherAfterSourceToggle(dateStr,i);
    };
    let lastTap=0;
    line.addEventListener("click",()=>{
      const now=Date.now();
      if(now-lastTap<420){ toggle(); lastTap=0; }
      else lastTap=now;
    });
    grid.appendChild(line);
  }
  card.appendChild(grid);
  body.appendChild(card);
}
function compactHourlyIndexes(idxs){
  const wide=(typeof window!=="undefined" && window.matchMedia && window.matchMedia("(min-width: 1200px) and (min-height: 760px)").matches);
  const limit=wide?10:6;
  if(idxs.length<=limit) return idxs.slice();
  const step=Math.max(1,Math.ceil(idxs.length/limit));
  return idxs.filter((_,pos)=>pos%step===0).slice(0,limit);
}

function weatherHourlyGridColumns(){
  // Weather day popup should always present hourly rows as a two-column
  // down-first grid. Earlier large-screen CSS/JS allowed three columns,
  // which made the reading order awkward and pushed source details aside.
  return 2;
}
function applyWeatherHourlyColumnOrder(rows,count){
  const cols=weatherHourlyGridColumns();
  rows.dataset.hourColumns=String(cols);
  if(cols>1){
    const rowCount=Math.max(1,Math.ceil(Math.max(1,count)/cols));
    rows.style.gridAutoFlow="column";
    rows.style.gridTemplateRows="repeat("+rowCount+", minmax(0, auto))";
  }else{
    rows.style.gridAutoFlow="";
    rows.style.gridTemplateRows="";
  }
}

function appendWeatherHourlySection(body,dateStr){
  if(!WX.hourly) return false;
  const idxs=[];
  WX.hourly.time.forEach((t,j)=>{ if(t.startsWith(dateStr)) idxs.push(j); });
  if(!idxs.length){
    const note=el("div"); note.style.marginTop="12px"; note.style.color="var(--dimmer)";
    note.textContent="Hourly detail isn't published this far out — showing daily summary only.";
    body.appendChild(note);
    return false;
  }
  const h=el("div","wxhourblock");
  const head=el("div","wxhourhead");
  head.appendChild(el("div",null,"Hourly"));
  const toggle=el("button","wxhourtoggle","Show hourly");
  toggle.type="button";
  toggle.setAttribute("aria-expanded","false");
  if(idxs.length>6) head.appendChild(toggle);
  h.appendChild(head);
  const rows=el("div","wxhourrows");
  h.appendChild(rows);
  const render=(expanded)=>{
    rows.replaceChildren();
    const show=expanded?idxs:compactHourlyIndexes(idxs);
    applyWeatherHourlyColumnOrder(rows,show.length);
    for(const j of show){
      const r=el("div","row hourrow wxhourrow");
      const hr=FMT.hour.format(new Date(WX.hourly.time[j]));
      const [,hic]=wmo(WX.hourly.weather_code?WX.hourly.weather_code[j]:0);
      const temp=Array.isArray(WX.hourly.temperature_2m)?wxDegree(WX.hourly.temperature_2m[j]):"—";
      const ppRaw=Array.isArray(WX.hourly.precipitation_probability)?WX.hourly.precipitation_probability[j]:null;
      const pp=ppRaw==null?"":wxPercent(ppRaw);
      const ppClass=Number(ppRaw)>=10?" wxhourprecip-on":"";
      const icon=el("span","wxhouricon"); icon.innerHTML=hic;
      r.append(el("span","wxhourtime",hr),icon,el("span","wxhourtemp",temp),el("span","wxhourprecip"+ppClass,pp));
      rows.appendChild(r);
    }
  };
  let expanded=false;
  render(false);
  toggle.addEventListener("click",()=>{
    expanded=!expanded;
    toggle.textContent=expanded?"Show fewer":"Show hourly";
    toggle.setAttribute("aria-expanded",expanded?"true":"false");
    render(expanded);
  });
  body.appendChild(h);
  return true;
}

function wxDegree(v){
  if(v==null || !Number.isFinite(+v)) return "—";
  return Math.round(Number(v))+"°";
}
function wxTimeOnly(v){
  if(!v) return ["—",""];
  const parts=FMT.time.format(new Date(v)).split(/\s+/);
  return [parts[0]||"—",parts.slice(1).join(" ")];
}
function wxPrecipTotalText(v){
  if(v==null || !Number.isFinite(+v)) return "— total";
  const mm=Number(v);
  if(CONFIG.tempUnit==="celsius") return wxNum(mm,1)+" mm total";
  return wxNum(mm/25.4,2)+'" total';
}
function wxMetricCard(row){
  const r=el("div","row wxsummaryrow"+(row.extraClass?" "+row.extraClass:""));
  r.innerHTML=wxMetricIcon(row.key);
  r.append(
    el("span","wxsummarylabel",String(row.label==null?"":row.label)),
    el("span","wxsummaryvalue"+(row.valueClass?" "+row.valueClass:""),String(row.value==null?"":row.value)),
    el("span","wxsummaryunit"+(row.unitClass?" "+row.unitClass:""),String(row.unit==null?"":row.unit))
  );
  return r;
}

const WX_METRIC_ICON_SVGS=Object.freeze({
  high:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M10 6v9\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M18 5v8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"m15 8 3-3 3 3\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  low:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M10 6v9\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M18 5v8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"m15 10 3 3 3-3\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  feels:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M14 14.8V5.5a4 4 0 1 0-8 0v9.3a6 6 0 1 0 8 0Z\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M10 7v8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M18.5 6.5c1.2 1 1.2 2.1 0 3.1s-1.2 2.1 0 3.1\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  precipChance:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M7 17.5c-2 0-3.6-1.5-3.6-3.4 0-1.8 1.4-3.3 3.2-3.4A5.8 5.8 0 0 1 18 10.2a3.6 3.6 0 0 1-.5 7.3H7Z\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M8 20 18 10\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><circle cx=\"8.5\" cy=\"11.5\" fill=\"currentColor\" r=\"1.1\"></circle><circle cx=\"17.5\" cy=\"18.5\" fill=\"currentColor\" r=\"1.1\"></circle></svg>",
  precipTotal:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M12 3.5C9 7.4 7 10.4 7 13.2a5 5 0 0 0 10 0c0-2.8-2-5.8-5-9.7Z\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M8 20h8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M18.5 7v10\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M17 9h3\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M17 15h3\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  wind:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M3 8h11.5a2.5 2.5 0 1 0-2.2-3.7\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M3 12h16.2a2.8 2.8 0 1 1-2.5 4\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M3 16h8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  uv:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><circle cx=\"12\" cy=\"12\" fill=\"none\" r=\"3.5\" stroke=\"currentColor\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></circle><path d=\"M12 2.5v2.2M12 19.3v2.2M4.6 4.6l1.6 1.6M17.8 17.8l1.6 1.6M2.5 12h2.2M19.3 12h2.2M4.6 19.4l1.6-1.6M17.8 6.2l1.6-1.6\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  sunrise:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M4 18h16\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M7 15a5 5 0 0 1 10 0\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M12 4v8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"m8.8 7.2 3.2-3.2 3.2 3.2\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  sunset:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><path d=\"M4 18h16\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M7 15a5 5 0 0 1 10 0\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"M12 4v8\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path><path d=\"m8.8 8.8 3.2 3.2 3.2-3.2\" fill=\"none\" stroke=\"currentColor\" stroke-linecap=\"round\" stroke-linejoin=\"round\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></path></svg>",
  other:"<svg aria-hidden=\"true\" class=\"wxstaticon\" focusable=\"false\" viewBox=\"0 0 24 24\"><circle cx=\"12\" cy=\"12\" fill=\"none\" r=\"7\" stroke=\"currentColor\" stroke-width=\"2.1\" vector-effect=\"non-scaling-stroke\"></circle></svg>",
});
function wxMetricIcon(kind){
  return WX_METRIC_ICON_SVGS[kind]||WX_METRIC_ICON_SVGS.other;
}
function showWxDayPopup(i){
  if(typeof setPopupMode==="function") setPopupMode("weatherpop");
  if(!WX) return;
  const d=WX.daily, day=new Date(d.time[i]+"T00:00");
  $("#poptitle").textContent=FMT.dayLong.format(day);
  const [desc,ic]=wmo(d.weather_code[i]);
  const when=$("#popwhen"), whenRow=el("span");
  whenRow.style.cssText="display:inline-flex;align-items:center;gap:8px";
  const whenIcon=el("span"); whenIcon.style.cssText="font-size:30px;line-height:1"; whenIcon.innerHTML=ic;
  whenRow.append(whenIcon,document.createTextNode(desc)); when.replaceChildren(whenRow);
  const body=$("#popbody"); body.replaceChildren();
  const high=Array.isArray(d.temperature_2m_max)?wxDegree(d.temperature_2m_max[i]):"—";
  const low=Array.isArray(d.temperature_2m_min)?wxDegree(d.temperature_2m_min[i]):"—";
  const feels=Array.isArray(d.apparent_temperature_max)?wxDegree(d.apparent_temperature_max[i]):"—";
  const precipChance=Array.isArray(d.precipitation_probability_max)?wxPercent(d.precipitation_probability_max[i]):"—";
  const precipTotal=Array.isArray(d.precipitation_sum)?wxPrecipTotalText(d.precipitation_sum[i]):"— total";
  const windValue=Array.isArray(d.wind_speed_10m_max)?wxNum(d.wind_speed_10m_max[i],0):"—";
  const uvRaw=Array.isArray(d.uv_index_max)?cleanUv(d.uv_index_max[i]):null;
  const uvInfo=uvRaw!=null?uvCategory(uvRaw):["",""];
  const sunrise=d.sunrise?wxTimeOnly(d.sunrise[i]):["—",""];
  const sunset=d.sunset?wxTimeOnly(d.sunset[i]):["—",""];

  const hero=el("div","wxhero"), heroTemps=el("div","wxherotemps"), heroMeta=el("div","wxherometa");
  heroTemps.append(el("span","wxherohigh",high),el("span","wxherolow","/ "+low));
  heroMeta.append(el("span",null,"High / low"),el("span",null,"Feels up to "+feels));
  hero.append(heroTemps,heroMeta); body.appendChild(hero);

  const rows=[
    {key:"precipTotal",label:"Precip",value:precipChance,unit:precipTotal},
    {key:"wind",label:"Wind",value:windValue,unit:CONFIG.windUnit+" max"},
    {key:"uv",label:"UV",value:uvRaw!=null?wxNum(uvRaw,1):"—",unit:uvInfo[0],unitClass:uvInfo[1]?"wxuvsev "+uvInfo[1]:""},
    {key:"sunrise",label:"Sunrise",value:sunrise[0],unit:sunrise[1]},
    {key:"sunset",label:"Sunset",value:sunset[0],unit:sunset[1]},
  ];
  const summary=el("div","wxsummarygrid");
  for(const row of rows) summary.appendChild(wxMetricCard(row));
  body.appendChild(summary);

  const detailWrap=el("div","wxdetailstack");
  appendWeatherHourlySection(detailWrap,d.time[i]);
  appendWeatherSourceDetails(detailWrap,i);
  appendWeatherSourceNotes(detailWrap,i);
  body.appendChild(detailWrap);
  openScrim();
}
