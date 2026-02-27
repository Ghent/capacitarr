import { ofetch } from 'ofetch'

export const useApi = () => {
    const config = useRuntimeConfig()

    const token = useCookie('jwt')

    const apiFetch = ofetch.create({
        baseURL: config.public.apiBaseUrl,
        onRequest({ request, options }) {
            if (token.value) {
                options.headers = options.headers || {}
                options.headers.Authorization = `Bearer ${token.value}`
            }
        },
        onResponseError({ response }) {
            if (response.status === 401) {
                // Handle unauthenticated behavior (e.g., redirect to login)
                const router = useRouter()
                token.value = null
                router.push('/login')
            }
        }
    })

    return apiFetch
}
